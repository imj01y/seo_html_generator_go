-- cache_handler.lua
-- Nginx Lua 缓存处理模块
-- 用于读取 Go 后端生成的 HTML 缓存文件，并记录蜘蛛访问日志

local _M = {}

-- 路径标准化（与 Go 的 normalizePath 保持一致）
function _M.normalize_path(path)
    -- 去除前导斜杠
    while string.sub(path, 1, 1) == "/" do
        path = string.sub(path, 2)
    end

    -- 空或根路径变成 index.html
    if path == "" then
        return "index.html"
    end

    -- 没有扩展名则加 .html
    if not string.match(path, "%.[^/]+$") then
        path = path .. ".html"
    end

    return path
end

-- 构建缓存文件路径
-- 与 Go 的 getCachePath 保持一致：{cache_dir}/{domain}/{hash[0:2]}/{hash[2:4]}/{normalized}
function _M.build_cache_path(cache_dir, domain, path)
    -- hash 是对原始 path 计算（包含前导斜杠，与 Go 保持一致）
    local path_hash = ngx.md5(path)
    local normalized = _M.normalize_path(path)

    -- 结构: {cache_dir}/{domain}/{hash[0:2]}/{hash[2:4]}/{normalized}
    return string.format("%s/%s/%s/%s/%s",
        cache_dir,
        domain,
        string.sub(path_hash, 1, 2),
        string.sub(path_hash, 3, 4),
        normalized
    )
end

-- 异步记录蜘蛛日志（使用 resty.dns.resolver 解析 + lua-resty-http 发送请求）
function _M.log_spider_async(domain, path, ua, ip, cache_hit, resp_time)
    -- 预先构建参数
    local query_str = ngx.encode_args({
        ua = ua,
        domain = domain,
        path = path,
        ip = ip,
        cache_hit = cache_hit and "1" or "0",
        resp_time = tostring(resp_time)
    })

    local ok, err = ngx.timer.at(0, function(premature)
        if premature then return end

        -- 使用 Docker 内置 DNS 解析 go-server
        local resolver = require "resty.dns.resolver"
        local r, err = resolver:new({
            nameservers = {"127.0.0.11"},
            retrans = 3,
            timeout = 2000,
        })

        if not r then
            ngx.log(ngx.WARN, "log spider failed to create resolver: ", err)
            return
        end

        local answers, err = r:query("go-server", { qtype = r.TYPE_A })
        if not answers then
            ngx.log(ngx.WARN, "log spider DNS query failed: ", err)
            return
        end

        if answers.errcode then
            ngx.log(ngx.WARN, "log spider DNS error: ", answers.errcode, " ", answers.errstr)
            return
        end

        -- 获取解析到的 IP
        local server_ip = nil
        for _, ans in ipairs(answers) do
            if ans.address then
                server_ip = ans.address
                break
            end
        end

        if not server_ip then
            ngx.log(ngx.WARN, "log spider no IP found for go-server")
            return
        end

        -- 使用 IP 地址连接
        local http = require "resty.http"
        local httpc = http.new()
        httpc:set_timeouts(3000, 5000, 5000)  -- connect, send, read (ms)

        local ok, err = httpc:connect(server_ip, tonumber(os.getenv("API_PORT") or "8080"))
        if not ok then
            ngx.log(ngx.WARN, "log spider connect failed: ", err)
            return
        end

        local res, err = httpc:request({
            method = "GET",
            path = "/api/log/spider?" .. query_str,
            headers = {
                ["Host"] = "go-server",
            }
        })

        if not res then
            ngx.log(ngx.WARN, "log spider request failed: ", err)
            return
        end

        -- 读取响应体（必须，否则连接无法复用）
        local body = res:read_body()

        if res.status >= 400 then
            ngx.log(ngx.WARN, "log spider bad status: ", res.status, " body: ", body)
        end

        -- 设置 keepalive
        httpc:set_keepalive(60000, 100)
    end)

    if not ok then
        ngx.log(ngx.ERR, "failed to create log spider timer: ", err)
    end
end

-- 读取缓存文件
function _M.read_cache_file(file_path)
    local file = io.open(file_path, "r")
    if not file then
        return nil
    end

    local content = file:read("*a")
    file:close()
    return content
end

return _M