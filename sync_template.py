#!/usr/bin/env python3
"""同步模板文件到数据库"""

import pymysql
from pathlib import Path
from dynaconf import Dynaconf

# 加载配置
settings = Dynaconf(
    settings_files=["config.yaml"],
    environments=True,
    env_switcher="ENV_FOR_DYNACONF",
)

# 数据库配置
db_config = {
    "host": settings.get("database.host", "localhost"),
    "port": settings.get("database.port", 3306),
    "user": settings.get("database.user", "root"),
    "password": settings.get("database.password", ""),
    "database": settings.get("database.database", "seo_html_generator"),
    "charset": "utf8mb4",
}

def sync_template():
    template_file = Path(__file__).parent / "database" / "templates" / "download_site.html"

    if not template_file.exists():
        print(f"Template file not found: {template_file}")
        return

    content = template_file.read_text(encoding="utf-8")
    print(f"Read template: {len(content)} characters")

    conn = pymysql.connect(**db_config)
    try:
        with conn.cursor() as cur:
            # 强制更新模板内容
            cur.execute(
                "UPDATE templates SET content = %s WHERE name = %s",
                (content, "download_site")
            )
            conn.commit()
            print(f"Updated {cur.rowcount} row(s)")
    finally:
        conn.close()

if __name__ == "__main__":
    sync_template()
    print("Done!")
