#!/bin/sh
set -e

PAGE_PORT=${PAGE_PORT:-8009}
ADMIN_PORT=${ADMIN_PORT:-8008}
API_PORT=${API_PORT:-8080}

# 从模板生成 nginx.conf
sed "s/\${PAGE_PORT}/$PAGE_PORT/g; s/\${ADMIN_PORT}/$ADMIN_PORT/g; s/\${API_PORT}/$API_PORT/g" \
  /etc/nginx/templates/nginx.conf.template > /usr/local/openresty/nginx/conf/nginx.conf

# 从模板生成 conf.d/*.conf
for f in /etc/nginx/templates/conf.d/*.template; do
  [ -f "$f" ] || continue
  filename=$(basename "$f" .template)
  sed "s/\${PAGE_PORT}/$PAGE_PORT/g; s/\${ADMIN_PORT}/$ADMIN_PORT/g; s/\${API_PORT}/$API_PORT/g" \
    "$f" > /etc/nginx/conf.d/"$filename"
done

# 复制用户自定义 .conf 文件（非模板，原样复制）
for f in /etc/nginx/templates/conf.d/*.conf; do
  [ -f "$f" ] || continue
  cp "$f" /etc/nginx/conf.d/
done

exec openresty -g 'daemon off;'
