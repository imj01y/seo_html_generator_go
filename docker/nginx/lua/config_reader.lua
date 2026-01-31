-- config_reader.lua
-- 从 config.yaml 读取缓存配置

local _M = {}

local CONFIG_FILE = "/app/config.yaml"
local DEFAULT_CACHE_DIR = "/data/cache"

-- 简单解析 YAML 中的 cache.dir 值
function _M.parse_cache_dir(content)
    -- 匹配 cache: 块下的 dir: 值
    -- 支持格式: dir: /data/cache 或 dir: "/data/cache"
    local in_cache_block = false
    for line in content:gmatch("[^\r\n]+") do
        -- 检查是否进入 cache: 块
        if line:match("^%s*cache:") then
            in_cache_block = true
        elseif in_cache_block then
            -- 检查是否是新的顶级块（不以空格开头且不是注释）
            if line:match("^%S") and not line:match("^#") then
                in_cache_block = false
            else
                -- 在 cache 块内查找 dir:
                local dir = line:match("^%s+dir:%s*[\"']?([^\"'%s#]+)")
                if dir then
                    return dir
                end
            end
        end
    end
    return nil
end

-- 读取配置文件
function _M.load_cache_dir()
    local file = io.open(CONFIG_FILE, "r")
    if not file then
        ngx.log(ngx.WARN, "config_reader: cannot open ", CONFIG_FILE)
        return DEFAULT_CACHE_DIR
    end

    local content = file:read("*a")
    file:close()

    local dir = _M.parse_cache_dir(content)
    if dir then
        -- 处理相对路径（如有）
        if dir:sub(1, 2) == "./" then
            dir = "/app/" .. dir:sub(3)
        elseif dir:sub(1, 1) ~= "/" then
            dir = "/app/" .. dir
        end
        ngx.log(ngx.INFO, "config_reader: cache_dir = ", dir)
        return dir
    end

    ngx.log(ngx.WARN, "config_reader: cache.dir not found, using default")
    return DEFAULT_CACHE_DIR
end

-- 获取缓存目录（使用 shared dict 缓存，避免每次请求都读文件）
function _M.get_cache_dir()
    local cache_config = ngx.shared.cache_config
    if cache_config then
        local dir = cache_config:get("cache_dir")
        if dir then
            return dir
        end
    end

    -- 首次加载或 shared dict 不可用
    local dir = _M.load_cache_dir()
    if cache_config then
        cache_config:set("cache_dir", dir)
    end
    return dir
end

return _M
