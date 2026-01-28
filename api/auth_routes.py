# -*- coding: utf-8 -*-
"""
认证相关 API 路由

包含登录、登出、用户信息获取、密码修改等功能。
"""
from fastapi import APIRouter, Depends

from api.deps import verify_token
from api.schemas import LoginRequest, LoginResponse, PasswordChangeRequest
from core.auth import (
    authenticate_admin, create_access_token, get_admin_by_username,
    update_admin_password
)

router = APIRouter(prefix="/api/auth", tags=["认证"])


@router.post("/login", response_model=LoginResponse)
async def login(request: LoginRequest):
    """
    管理员登录

    从数据库验证用户名和密码，成功后返回 JWT Token。
    """
    admin = await authenticate_admin(request.username, request.password)

    if not admin:
        return LoginResponse(success=False, message="用户名或密码错误")

    token = create_access_token(data={
        "sub": admin['username'],
        "admin_id": admin['id'],
        "role": admin['role']
    })

    return LoginResponse(
        success=True,
        token=token,
        message="登录成功"
    )


@router.post("/logout")
async def logout(token_data: dict = Depends(verify_token)):
    """退出登录"""
    return {"success": True}


@router.get("/profile")
async def get_profile(token_data: dict = Depends(verify_token)):
    """获取当前用户信息"""
    username = token_data.get('sub', 'unknown')
    admin = await get_admin_by_username(username)

    if admin:
        return {
            "id": admin['id'],
            "username": admin['username'],
            "role": "admin",
            "last_login": admin['last_login'].isoformat() if admin['last_login'] else None
        }

    return {
        "username": username,
        "role": token_data.get('role', 'admin'),
        "last_login": None
    }


@router.post("/change-password")
async def change_password(
    request: PasswordChangeRequest,
    token_data: dict = Depends(verify_token)
):
    """
    修改密码

    需要验证旧密码，然后设置新密码。
    """
    username = token_data.get('sub')
    admin_id = token_data.get('admin_id')

    if not username or not admin_id:
        return {"success": False, "message": "Invalid token data"}

    admin = await authenticate_admin(username, request.old_password)
    if not admin:
        return {"success": False, "message": "旧密码错误"}

    if len(request.new_password) < 6:
        return {"success": False, "message": "新密码长度至少6位"}

    success = await update_admin_password(admin_id, request.new_password)

    if success:
        return {"success": True, "message": "密码修改成功"}
    else:
        return {"success": False, "message": "密码修改失败"}
