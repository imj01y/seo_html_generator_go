"""
认证模块

提供用户认证相关功能：
- 密码哈希和验证
- JWT Token 生成和验证
- 默认管理员创建
- 密码修改

使用方法:
    from core.auth import verify_password, hash_password, create_access_token

    # 哈希密码
    hashed = hash_password("plain_password")

    # 验证密码
    is_valid = verify_password("plain_password", hashed)

    # 创建JWT Token
    token = create_access_token({"sub": "username"})
"""
from datetime import datetime, timedelta
from typing import Optional, Dict, Any

import bcrypt
import jwt
from loguru import logger

from config import get_config


def hash_password(password: str) -> str:
    """
    哈希密码

    Args:
        password: 明文密码

    Returns:
        bcrypt 哈希后的密码字符串
    """
    salt = bcrypt.gensalt(rounds=12)
    hashed = bcrypt.hashpw(password.encode('utf-8'), salt)
    return hashed.decode('utf-8')


def verify_password(plain_password: str, hashed_password: str) -> bool:
    """
    验证密码

    Args:
        plain_password: 明文密码
        hashed_password: 哈希后的密码

    Returns:
        密码是否匹配
    """
    try:
        return bcrypt.checkpw(
            plain_password.encode('utf-8'),
            hashed_password.encode('utf-8')
        )
    except Exception as e:
        logger.warning(f"Password verification failed: {e}")
        return False


def create_access_token(data: Dict[str, Any], expires_delta: Optional[timedelta] = None) -> str:
    """
    创建 JWT Access Token

    Args:
        data: Token 数据（如 {"sub": "username"}）
        expires_delta: 过期时间增量

    Returns:
        JWT Token 字符串
    """
    config = get_config()
    to_encode = data.copy()

    if expires_delta:
        expire = datetime.utcnow() + expires_delta
    else:
        expire_minutes = getattr(config.auth, 'access_token_expire_minutes', 1440)
        expire = datetime.utcnow() + timedelta(minutes=expire_minutes)

    to_encode.update({"exp": expire, "iat": datetime.utcnow()})

    secret_key = getattr(config.auth, 'secret_key', 'default-secret-key')
    algorithm = getattr(config.auth, 'algorithm', 'HS256')

    encoded_jwt = jwt.encode(to_encode, secret_key, algorithm=algorithm)
    return encoded_jwt


def verify_token(token: str) -> Optional[Dict[str, Any]]:
    """
    验证 JWT Token

    Args:
        token: JWT Token 字符串

    Returns:
        Token 数据字典，验证失败返回 None
    """
    config = get_config()
    secret_key = getattr(config.auth, 'secret_key', 'default-secret-key')
    algorithm = getattr(config.auth, 'algorithm', 'HS256')

    try:
        payload = jwt.decode(token, secret_key, algorithms=[algorithm])
        return payload
    except jwt.ExpiredSignatureError:
        logger.warning("Token has expired")
        return None
    except jwt.InvalidTokenError as e:
        logger.warning(f"Invalid token: {e}")
        return None


async def get_admin_by_username(username: str) -> Optional[Dict[str, Any]]:
    """
    根据用户名获取管理员信息

    Args:
        username: 用户名

    Returns:
        管理员信息字典，不存在返回 None
    """
    from database.db import fetch_one

    try:
        admin = await fetch_one(
            "SELECT id, username, password, last_login FROM admins WHERE username = %s",
            (username,)
        )
        return admin
    except Exception as e:
        logger.error(f"Failed to get admin by username: {e}")
        return None


async def authenticate_admin(username: str, password: str) -> Optional[Dict[str, Any]]:
    """
    验证管理员登录

    Args:
        username: 用户名
        password: 密码

    Returns:
        管理员信息字典（不含密码），验证失败返回 None
    """
    admin = await get_admin_by_username(username)
    if not admin:
        logger.warning(f"Admin not found: {username}")
        return None

    if not verify_password(password, admin['password']):
        logger.warning(f"Invalid password for admin: {username}")
        return None

    # 更新最后登录时间
    from database.db import execute_query
    try:
        await execute_query(
            "UPDATE admins SET last_login = NOW() WHERE id = %s",
            (admin['id'],)
        )
    except Exception as e:
        logger.warning(f"Failed to update last login time: {e}")

    # 返回不含密码的信息（role 字段已移除，默认为 admin）
    return {
        'id': admin['id'],
        'username': admin['username'],
        'role': 'admin',
        'last_login': admin['last_login']
    }


async def create_admin(username: str, password: str) -> Optional[int]:
    """
    创建管理员账号

    Args:
        username: 用户名
        password: 明文密码

    Returns:
        新建管理员的ID，失败返回 None
    """
    from database.db import insert

    hashed_password = hash_password(password)

    try:
        admin_id = await insert('admins', {
            'username': username,
            'password': hashed_password
        })
        logger.info(f"Admin created: {username} (ID: {admin_id})")
        return admin_id
    except Exception as e:
        if "Duplicate" in str(e):
            logger.debug(f"Admin already exists: {username}")
        else:
            logger.error(f"Failed to create admin: {e}")
        return None


async def update_admin_password(admin_id: int, new_password: str) -> bool:
    """
    更新管理员密码

    Args:
        admin_id: 管理员ID
        new_password: 新密码（明文）

    Returns:
        是否更新成功
    """
    from database.db import execute_query

    password_hash = hash_password(new_password)

    try:
        await execute_query(
            "UPDATE admins SET password = %s WHERE id = %s",
            (password_hash, admin_id)
        )
        logger.debug(f"Password updated for admin ID: {admin_id}")
        return True
    except Exception as e:
        logger.error(f"Failed to update password: {e}")
        return False


async def ensure_default_admin():
    """
    确保默认管理员存在且密码正确

    从配置文件读取默认账号信息：
    - 如果数据库中不存在则创建
    - 如果存在但密码不匹配（如 schema.sql 中的旧哈希），则更新密码
    """
    config = get_config()

    # 获取默认管理员配置
    default_admin = getattr(config.auth, 'default_admin', None)
    if not default_admin:
        logger.warning("No default admin configured in config.yaml")
        return

    username = getattr(default_admin, 'username', 'admin')
    password = getattr(default_admin, 'password', 'admin_6yh7uJ')

    # 检查是否已存在
    existing = await get_admin_by_username(username)
    if existing:
        # 验证密码是否正确
        if verify_password(password, existing['password']):
            logger.debug(f"Default admin '{username}' verified")
        else:
            # 密码不匹配，静默更新为配置文件中的密码（常见于首次启动或密码变更）
            logger.debug(f"Syncing default admin '{username}' password with config")
            success = await update_admin_password(existing['id'], password)
            if not success:
                logger.error(f"Failed to update default admin '{username}' password")
        return

    # 创建默认管理员
    admin_id = await create_admin(username, password)
    if admin_id:
        logger.info(f"Default admin '{username}' created successfully")
    else:
        logger.warning(f"Failed to create default admin '{username}'")
