"""
配置管理模块

使用 Dynaconf 加载配置，支持:
- YAML 配置文件
- 环境变量覆盖 (前缀: SEO_)
- 多环境配置 (development, production)
- .secrets.yaml 敏感信息分离

使用方法:
    from config import settings

    # 访问配置
    port = settings.server.port
    db_host = settings.database.host

    # 环境变量覆盖
    # SEO_SERVER__PORT=9000 会覆盖 server.port
"""
from pathlib import Path
from dynaconf import Dynaconf

# 配置文件目录
_config_dir = Path(__file__).parent

# 创建 Dynaconf 实例
settings = Dynaconf(
    # 环境变量前缀，如 SEO_SERVER__PORT=8000
    envvar_prefix="SEO",

    # 配置文件列表（按顺序加载，后面的覆盖前面的）
    settings_files=[
        str(_config_dir / "config.yaml"),
        str(_config_dir / ".secrets.yaml"),  # 可选，存放敏感信息
    ],

    # 启用环境支持 [development], [production]
    environments=True,

    # 默认环境
    env="development",

    # 加载 .env 文件
    load_dotenv=True,

    # 环境变量中嵌套配置使用双下划线
    # SEO_DATABASE__HOST=localhost
    nested_separator="__",

    # 合并嵌套配置而非覆盖
    merge_enabled=True,
)


# ============================================
# 兼容层：保持旧API可用
# ============================================

def get_config():
    """获取配置（兼容旧代码）"""
    return settings


def reload_config(path: str = None):
    """重新加载配置"""
    settings.reload()
    return settings


# 导出类型别名（兼容旧代码的类型标注）
Config = type(settings)
