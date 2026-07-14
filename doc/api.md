
# Gooj 评测系统 API 文档

## 概述

本文档详细描述了 Gooj 评测系统的所有 API 接口，包括接口路径、HTTP 方法、URL 参数、请求体格式、响应格式、权限要求以及前端使用情况。

**基础 URL**: `/`

**认证方式**: 通过 Cookie 中的 Session Token 进行认证。管理类 API 需要登录且具有相应权限。

**通用响应格式**: 所有 API 均返回 JSON 格式数据。

---

## 权限说明

系统采用基于用户组的权限控制模型，主要权限包括：

| 权限名称 | 说明 |
|---------|------|
| `EditPermission` | 编辑题目、管理比赛权限 |
| `UserPermission` | 用户管理权限 |
| `GroupPermission` | 用户组管理权限 |

**权限检查中间件**: `manage.AuthMiddleWare` 会应用于所有路由（除公开路由外），未登录用户访问受保护 API 将返回 401 Unauthorized。

**公开路由**（无需登录）：
- `/board` [GET]
- `/message` [POST]
- `/problem` [GET]
- `/problemlist` [GET]
- `/contest` [GET]
- `/contest/{id}` [GET]
- `/contest/{id}/leaderboard` [GET]
- `/ranking` [GET]
- `/register` [GET/POST]
- `/login` [POST]
- `/user/{username}` [GET]
- `/problems` [GET]
- `/api/problem/{id}` [GET] (受时间限制，未公开题目需登录)
- `/api/contests` [GET]
- `/api/contest/{id}` [GET]
- `/api/contest/{id}/leaderboard` [GET]
- `/api/rankings` [GET]
- `/api/groups` [GET]
- `/api/allUsers` [GET]

---

## 消息与公告板 API

### `/message` [POST]

**功能**: 向公告板发送一条新消息。

**权限**: 公开（无需登录）

**输入**:
- Content-Type: `application/json` 或 `application/x-www-form-urlencoded`
- JSON 字段: `{ "message": "string" }`
- 表单字段: `message`

**输出**:
```json
{ "status": "ok" }
```
- 200 OK: 成功
- 400 Bad Request: 空消息或缺少字段
- 500 Internal Server Error: 服务器内部错误

**前端使用**: 未被当前前端页面直接使用（后端功能）

---

### `/message/{index}` [DELETE]

**功能**: 删除公告板上指定索引的消息。

**权限**: 需要登录

**输入**:
- URL 参数: `index` (int, 从 0 开始)

**输出**:
```json
{ "status": "ok" }
```
- 200 OK: 成功
- 400 Bad Request: 无效索引
- 401 Unauthorized: 未登录

**前端使用**: 未被前端页面使用

---

### `/board` [GET]

**功能**: 获取公告板所有消息。

**权限**: 公开（无需登录）

**输入**: 无

**输出**:
```json
{
  "messages": ["message1", "message2", ...]
}
```

**前端使用**: 未被前端页面使用（可能用于后端内部调用）

---

## 题目相关 API

### `/problems` [GET]

**功能**: 获取题目列表（分页）。

**权限**: 公开（未登录用户只能看到已公开题目）

**输入** (Query Parameters):
- `page` (int, 可选, 默认 1): 页码，从 1 开始
- `per` (int, 可选, 默认 10): 每页数量

**输出**:
```json
{
  "problems": [
    {
      "id": 1,
      "title": "A+B Problem",
      "tests_count": 10,
      "time_limit_ms": 1000,
      "mem_limit_mb": 256,
      "problem_visible": true,
      "test_visible": false
    }
  ],
  "total": 100,
  "page": 1,
  "per": 10
}
```

**说明**: 题目列表不包含 `description` 字段（避免传输大量数据），如需获取完整题目描述请使用 `/api/problem/{id}` 接口。

**前端使用**:
- [problemlist.html](static/problemlist.html): 加载题目列表
- [code.html](static/code.html): 加载题目列表供选择

---

### `/api/problem/{id}` [GET]

**功能**: 获取题目的描述和配置信息。

**权限**: 公开（题目在公开时间后可见，未公开题目需 EditPermission）

**输入**:
- URL 参数: `id` (string/int): 题目编号（仅支持数字 ID）

**输出**:
```json
{
  "id": 1,
  "title": "A+B Problem",
  "description": "...",
  "time_limit_ms": 1000,
  "mem_limit_mb": 256,
  "tests_count": 10,
  "public_time": "2024-01-01T00:00:00Z",
  "test_visible": false,
  "statement": "# 题目描述\n...",
  "statement_html": "<h1>题目描述</h1>...",
  "config": {
    "time_limit": 1000,
    "memory_limit": 256,
    "test_cases": [...]
  }
}
```

**前端使用**:
- [submit.html](static/submit.html): 加载题目详情
- [problem.html](static/problem.html): 显示题目内容
- [edit.html](static/edit.html): 编辑题目时加载题目数据

---

### `/api/problem/{id}/update` [POST]

**功能**: 更新题目元数据。

**权限**: 需要 `EditPermission`

**输入**:
- URL 参数: `id` (int): 题目编号
- JSON Body:
```json
{
  "title": "新标题",
  "description": "新描述",
  "time_limit_ms": 2000,
  "mem_limit_mb": 512,
  "public_time": "2024-01-01T00:00:00Z",
  "test_visible": true
}
```

**输出**:
```json
{ "status": "ok" }
```
- 200 OK: 成功
- 400 Bad Request: 无效请求体
- 403 Forbidden: 无权限
- 404 Not Found: 题目不存在

**前端使用**:
- [edit.html](static/edit.html): 保存题目修改

---

### `/api/problem_stats` [GET]

**功能**: 获取题目的统计信息（通过数、提交数等）。

**权限**: 公开

**输入** (Query Parameters):
- `problem` (string): 题目编号（仅支持数字 ID）

**输出**:
```json
{
  "problem_id": 1,
  "total_submissions": 100,
  "accepted_count": 50,
  "acceptance_rate": 0.5
}
```

**前端使用**:
- [problemlist.html](static/problemlist.html): 显示每道题的统计信息

---

## 提交与评测 API

### `/submit` [GET]

**功能**: 获取代码提交页面。

**权限**: 公开

**输入**: 无

**输出**: HTML 页面 (`static/submit.html`)

---

### `/submit` [POST]

**功能**: 提交代码进行评测。

**权限**: 需要登录

**输入** (JSON Body):
```json
{
  "username": "user1",
  "problem": "1",
  "code": "#include <bits/stdc++.h>\n..."
}
```

**输出**:
```json
{
  "status": "queued",
  "submission_id": 123
}
```

**前端使用**:
- [submit.html](static/submit.html): 提交代码
- [code.html](static/code.html): 提交代码

---

### `/last_submission` [GET]

**功能**: 获取某用户对某题目的最近一次提交及其评测结果。

**权限**: 需要登录（可查看自己的提交，或具有 EditPermission 可查看所有）

**输入** (Query Parameters):
- `username` (string): 用户名
- `problem` (string): 题目编号（仅支持数字 ID）

**输出**:
```json
{
  "submission": {
    "submission_id": 123,
    "username": "user1",
    "problem_id": 1,
    "code": "#include <bits/stdc++.h>\n...",
    "status": "accepted",
    "score": 100,
    "time_ms": 50,
    "memory_kb": 1024,
    "compileError": ""
  },
  "results": [
    {
      "test_index": 1,
      "passed": true,
      "time_ms": 10,
      "memory_kb": 512,
      "output": ""
    }
  ]
}
```

**前端使用**:
- [submit.html](static/submit.html): 显示最近一次提交结果

---

### `/result/{user}/{problem}` [GET]

**功能**: 获取某用户某题目的评测结果明细。

**权限**: 需要登录

**输入**:
- URL 参数: `user` (string): 用户名
- URL 参数: `problem` (string): 题目编号（仅支持数字 ID）

**输出**: 评测结果内容或错误信息

**前端使用**:
- [code.html](static/code.html): 显示评测结果

---

### `/codefile/{user}/{problem}` [GET]

**功能**: 获取某用户某题目的最后一次提交代码及评测摘要。

**权限**: 需要登录（受 TestVisible 限制）

**输入**:
- URL 参数: `user` (string): 用户名
- URL 参数: `problem` (string): 题目编号（仅支持数字 ID）

**输出**:
```json
{
  "code": "#include <bits/stdc++.h>\n...",
  "summary": {
    "status": "accepted",
    "test_results": [...]
  }
}
```

**前端使用**:
- [code.html](static/code.html): 显示代码内容

---

### `/submissions` [GET]

**功能**: 获取提交记录列表页面。

**权限**: 公开

**输入**: 无

**输出**: HTML 页面 (`static/submissions.html`)

---

### `/submission/{id}` [GET]

**功能**: 获取指定提交的详情页面。

**权限**: 公开

**输入**:
- URL 参数: `id` (int): 提交 ID

**输出**: HTML 页面 (`static/submission_detail.html`)

---

### `/api/submissions` [GET]

**功能**: 获取提交记录列表（分页）。

**权限**: 需要登录（普通用户只能查看自己的提交，EditPermission 可查看所有）

**输入** (Query Parameters):
- `page` (int, 可选, 默认 1): 页码
- `limit` (int, 可选, 默认 20, 最大 100): 每页数量
- `problem` (string, 可选): 题目编号（仅支持数字 ID）
- `username` (string, 可选): 用户名

**输出**:
```json
{
  "total": 100,
  "page": 1,
  "limit": 20,
  "submissions": [
    {
      "ID": 123,
      "Username": "user1",
      "ProblemID": 1,
      "Status": "accepted",
      "Score": 100,
      "TimeMs": 50,
      "MemoryKB": 1024,
      "CreatedAt": "2024-01-01T00:00:00Z"
    }
  ]
}
```

**前端使用**: 未被当前前端直接使用（页面使用静态 HTML）

---

### `/api/submission/{id}` [GET]

**功能**: 获取指定提交的详细信息。

**权限**: 需要登录（仅能查看自己的提交或具有 EditPermission 可查看所有）

**输入**:
- URL 参数: `id` (int): 提交 ID

**输出**: 提交详情 JSON

**说明**: 
- `test_results[].output`: 已移除（避免泄露测试数据）

---

## 用户与认证 API

### `/register` [GET]

**功能**: 获取用户注册页面。

**权限**: 公开

**输入**: 无

**输出**: HTML 页面 (`static/register.html`)

---

### `/register` [POST]

**功能**: 用户注册。

**权限**: 公开

**输入** (JSON Body):
```json
{
  "username": "newuser",
  "password": "password123",
  "group": "student"
}
```

**输出**:
```json
{
  "status": "ok",
  "message": "Registration submitted for approval"
}
```
- 200 OK: 注册成功（待审核）
- 400 Bad Request: 用户名已存在、组不存在或字段缺失

**前端使用**:
- [register.html](static/register.html): 用户注册表单

---

### `/login` [POST]

**功能**: 用户登录。

**权限**: 公开

**输入** (JSON Body):
```json
{
  "username": "user1",
  "password": "password123"
}
```

**输出**:
```json
{
  "status": "ok",
  "username": "user1"
}
```

**前端使用**: 未被前端直接使用（由前端 JS 处理）

---

### `/user/{username}` [GET]

**功能**: 获取用户公开信息页面。

**权限**: 公开

**输入**:
- URL 参数: `username` (string): 用户名

**输出**: HTML 页面 (`static/user_profile.html`)

---

### `/user_profile` [GET]

**功能**: 获取当前登录用户个人页面（重定向到 /user/{username}）。

**权限**: 需要登录

**输入**: 无

**输出**: HTTP 302 重定向到 `/user/{当前用户名}`

---

### `/api/user/{username}` [GET]

**功能**: 获取用户详细信息 JSON。

**权限**: 公开（只能查看公开信息）

**输入**:
- URL 参数: `username` (string): 用户名

**输出**:
```json
{
  "username": "user1",
  "group_name": "student",
  "rating": 1500,
  "role": "user",
  "solved_count": 10,
  "solved_ids": [1, 2, 3, ...],
  "total_submissions": 100,
  "accepted_submissions": 50,
  "created_at": "2024-01-01 00:00:00",
  "contests": [
    {
      "contest_name": "Weekly Contest 1",
      "contest_id": 1,
      "rank": 5,
      "total_score": 400,
      "rating_before": 1400,
      "rating_after": 1520,
      "rating_change": 120
    }
  ]
}
```

---

### `/api/user/{username}/rating` [POST]

**功能**: 更新用户 Rating。

**权限**: 用户只能更新自己的 Rating，或需要 `EditPermission`

**输入**:
- URL 参数: `username` (string): 用户名
- JSON Body: `{ "rating": 1600 }`

**输出**:
```json
{ "status": "ok" }
```

---

### `/api/user/{username}/submissions` [GET]

**功能**: 获取用户的提交记录。

**权限**: 公开

**输入**:
- URL 参数: `username` (string): 用户名

**输出**: 用户提交记录列表

---

### `/api/user/{username}/solved` [GET]

**功能**: 获取用户已解决的题目列表。

**权限**: 公开

**输入**:
- URL 参数: `username` (string): 用户名

**输出**: 已解决题目 ID 列表

---

### `/api/user/{username}/bio` [GET]

**功能**: 获取用户个人简介（Markdown 格式）。

**权限**: 用户本人或具有 `EditPermission` 的用户可查看

**输入**:
- URL 参数: `username` (string): 用户名

**输出**:
```json
{
  "bio": "## 个人简介\n\n这是我的简介内容，支持 Markdown 格式。"
}
```

**错误响应**:
- 403 Forbidden: 无权限查看该用户的简介

---

### `/api/user/{username}/bio` [POST]

**功能**: 更新用户个人简介（Markdown 格式）。

**权限**: 用户本人或具有 `EditPermission` 的用户可更新

**输入**:
- URL 参数: `username` (string): 用户名
- JSON Body: `{ "bio": "## 我的简介\n\n支持 Markdown 和 LaTeX 公式。" }`
- 简介长度限制: 最大 100KB

**输出**:
```json
{ "message": "Bio updated" }
```

**错误响应**:
- 400 Bad Request: 简介超过 100KB 限制
- 403 Forbidden: 无权限更新该用户的简介

---

## 用户管理 API (需要权限)

### `/api/users` [GET]

**功能**: 获取用户列表（按创建者筛选）。

**权限**: 需要登录

**输出**:
```json
{
  "users": [
    {
      "id": 1,
      "username": "user1",
      "group_name": "student",
      "rating": 1500,
      "role": "user",
      "created_at": "2024-01-01T00:00:00Z",
      "approved": true,
      "approved_at": "2024-01-02T00:00:00Z",
      "approved_by": "admin"
    }
  ],
  "total": 100
}
```

**说明**: 返回的用户信息已过滤敏感字段（不包含 `password`、`bio` 等内部字段）。

---

### `/api/allUsers` [GET]

**功能**: 获取所有已审核通过的用户列表。

**权限**: 公开

**输出**:
```json
{
  "users": [
    {
      "id": 1,
      "username": "user1",
      "group_name": "student",
      "rating": 1500,
      "role": "user",
      "created_at": "2024-01-01T00:00:00Z"
    }
  ],
  "total": 100
}
```

**说明**: 返回的用户信息已过滤敏感字段（不包含 `password`、`approved` 等内部字段）。

**前端使用**:
- [register.html](static/register.html): 注册时选择用户组
- [create_group.html](static/create_group.html): 显示所有用户列表

---

### `/api/groups` [GET]

**功能**: 获取所有用户分组信息。

**权限**: 公开

**输出**:
```json
{
  "groups": [
    {
      "ID": 1,
      "Name": "student",
      "CreatedBy": "admin",
      "EditPermission": false,
      "UserPermission": false,
      "GroupPermission": false
    }
  ],
  "total": 5
}
```

**前端使用**:
- [register.html](static/register.html): 注册时选择用户组
- [create_user.html](static/create_user.html): 创建用户时选择用户组
- [create_group.html](static/create_group.html): 显示所有分组

---

### `/api/user_permissions` [GET]

**功能**: 获取指定用户的权限信息。

**权限**: 公开

**输入** (Query Parameters):
- `username` (string): 用户名
- `permission` (string): 权限名称

**输出**:
```json
{
  "permited": true
}
```

**前端使用**:
- [problemlist.html](static/problemlist.html): 检查用户编辑权限
- [problem.html](static/problem.html): 检查用户编辑权限

---

### `/api/pending_users` [GET]

**功能**: 获取待审核用户列表。

**权限**: 需要 `UserPermission` 或 `GroupPermission`

**输出**:
```json
{
  "users": [
    {
      "id": 1,
      "username": "newuser",
      "group_name": "student",
      "rating": 1500,
      "role": "user",
      "created_at": "2024-01-01T00:00:00Z"
    }
  ],
  "total": 10
}
```

**说明**: 返回的用户信息已过滤敏感字段（不包含 `password` 等内部字段）。

**前端使用**:
- [manage_users.html](static/manage_users.html): 显示待审核用户

---

### `/api/approved_users` [GET]

**功能**: 获取已审核通过的用户列表。

**权限**: 需要 `UserPermission` 或 `GroupPermission`

**输出**:
```json
{
  "users": [
    {
      "id": 1,
      "username": "user1",
      "group_name": "student",
      "rating": 1500,
      "role": "user",
      "created_at": "2024-01-01T00:00:00Z"
    }
  ],
  "total": 90
}
```

**说明**: 返回的用户信息已过滤敏感字段（不包含 `password` 等内部字段）。

**前端使用**:
- [manage_users.html](static/manage_users.html): 显示已审核用户

---

### `/api/approve/{username}` [POST]

**功能**: 审核通过指定用户。

**权限**: 需要 `UserPermission` 或用户是组创建者

**输入**:
- URL 参数: `username` (string): 用户名

**输出**:
```json
{ "status": "ok" }
```

---

### `/api/reject/{username}` [POST]

**功能**: 拒绝（删除）指定用户的注册申请。

**权限**: 需要 `UserPermission` 或用户是组创建者

**输入**:
- URL 参数: `username` (string): 用户名

**输出**:
```json
{ "status": "ok" }
```

---

### `/api/create_user` [POST]

**功能**: 管理员创建新用户。

**权限**: 需要登录

**输入** (JSON Body):
```json
{
  "username": "newuser",
  "group": "student",
  "permissions": "optional"
}
```

**输出**:
```json
{
  "status": "ok",
  "password": "Abc12345"
}
```

**前端使用**:
- [create_user.html](static/create_user.html): 创建用户表单

---

### `/api/create_group` [POST]

**功能**: 管理员创建新用户组。

**权限**: 需要 `GroupPermission`

**输入** (JSON Body):
```json
{
  "groupName": "newgroup",
  "permissions": ["EditPermission", "UserPermission"]
}
```

**输出**:
```json
{ "status": "ok" }
```

**前端使用**:
- [create_group.html](static/create_group.html): 创建用户组

---

### `/api/update_group_creator` [POST]

**功能**: 更新用户组的创建者。

**权限**: 需要是组创建者或具有 `GroupPermission`

**输入** (JSON Body):
```json
{
  "groupName": "group1",
  "newCreator": "newadmin"
}
```

**输出**:
```json
{ "status": "ok" }
```

**前端使用**:
- [create_group.html](static/create_group.html): 更新组创建者

---

### `/api/delete_group` [POST]

**功能**: 删除用户组。

**权限**: 需要 `GroupPermission` 或是组创建者

**输入** (JSON Body):
```json
{
  "groupName": "group1"
}
```

**输出**:
```json
{ "status": "ok" }
```

**前端使用**:
- [create_group.html](static/create_group.html): 删除用户组

---

### `/api/reset_password` [POST]

**功能**: 重置用户密码。

**权限**: 需要 `UserPermission` 或是用户创建者

**输入** (JSON Body):
```json
{
  "username": "user1"
}
```

**输出**:
```json
{
  "status": "ok",
  "password": "NewPass123"
}
```

---

### `/api/delete_user` [POST]

**功能**: 删除用户。

**权限**: 需要 `UserPermission` 或是用户创建者

**输入** (JSON Body):
```json
{
  "username": "user1"
}
```

**输出**:
```json
{ "status": "ok" }
```

---

### `/api/import_users_csv` [POST]

**功能**: 批量导入用户（CSV 格式）。

**权限**: 需要 `UserPermission`

**输入**: Multipart Form
- `csv` 文件字段：CSV 文件，包含 `username`, `group`, `password` 列

**输出**:
```json
{
  "status": "ok",
  "imported": 50
}
```

---

## 比赛相关 API

### `/contest` [GET]

**功能**: 获取比赛列表页面。

**权限**: 公开

**输入**: 无

**输出**: HTML 页面 (`static/contest.html`)

---

### `/contest/{id}` [GET]

**功能**: 获取比赛详情页面。

**权限**: 公开

**输入**:
- URL 参数: `id` (int): 比赛 ID

**输出**: HTML 页面 (`static/contest_detail.html`)

---

### `/contest/{id}/leaderboard` [GET]

**功能**: 获取比赛排行榜页面。

**权限**: 公开

**输入**:
- URL 参数: `id` (int): 比赛 ID

**输出**: HTML 页面 (`static/contest_leaderboard.html`)

---

### `/api/contests` [GET]

**功能**: 获取所有比赛列表（含关联题目）。

**权限**: 公开

**输入**: 无

**输出**:
```json
{
  "contests": [
    {
      "id": 1,
      "title": "Weekly Contest 1",
      "description": "...",
      "start_at": "2024-01-01T00:00:00Z",
      "end_at": "2024-01-08T00:00:00Z",
      "created_by": "admin",
      "problems": [
        {
          "id": 1,
          "title": "A+B Problem",
          "tests_count": 10,
          "time_limit_ms": 1000,
          "mem_limit_mb": 256
        }
      ]
    }
  ],
  "total": 5
}
```

**说明**: 比赛中的题目列表只包含基本信息，不包含 `description` 字段。

**前端使用**:
- [contest.html](static/contest.html): 加载比赛列表

---

### `/api/contest/{id}` [GET]

**功能**: 获取单个比赛的详细信息。

**权限**: 公开

**输入**:
- URL 参数: `id` (int): 比赛 ID

**输出**:
```json
{
  "id": 1,
  "title": "Weekly Contest 1",
  "description": "...",
  "start_at": "2024-01-01T00:00:00Z",
  "end_at": "2024-01-08T00:00:00Z",
  "created_by": "admin",
  "problems": [
    {
      "id": 1,
      "title": "A+B Problem",
      "tests_count": 10,
      "time_limit_ms": 1000,
      "mem_limit_mb": 256
    }
  ]
}
```

**说明**: 比赛中的题目列表只包含基本信息，不包含 `description` 字段。

**前端使用**:
- [contest_detail.html](static/contest_detail.html): 加载比赛详情

---

### `/api/contest/{id}/leaderboard` [GET]

**功能**: 获取比赛的排行榜数据。

**权限**: 公开

**输入**:
- URL 参数: `id` (int): 比赛 ID

**输出**:
```json
{
  "contest_id": 1,
  "rows": [
    {
      "username": "user1",
      "group_name": "student",
      "rating": 1600,
      "scores": { "1": 100, "2": 100, "3": 100 },
      "total": 300
    }
  ]
}
```

---

### `/api/create_contest` [POST]

**功能**: 创建新比赛。

**权限**: 需要 `EditPermission`

**输入** (JSON Body):
```json
{
  "title": "Weekly Contest 2",
  "description": "...",
  "start_at": "2024-01-01T00:00:00Z",
  "end_at": "2024-01-08T00:00:00Z",
  "groups": ["student", "teacher"],
  "problem_ids": [1, 2, 3]
}
```

**输出**:
```json
{
  "status": "ok",
  "contest": {...}
}
```

---

### `/api/delete_contest` [POST]

**功能**: 删除比赛。

**权限**: 需要 `EditPermission`

**输入** (JSON Body):
```json
{
  "contest_id": 1
}
```

**输出**:
```json
{ "status": "ok" }
```

---

## 排行榜与 Rating API

### `/ranking` [GET]

**功能**: 获取全球 Rating 排行榜页面。

**权限**: 公开

**输入**: 无

**输出**: HTML 页面 (`static/rankings.html`)

---

### `/api/rankings` [GET]

**功能**: 获取 Rating 排行榜数据。

**权限**: 公开

**输入**: 无

**输出**:
```json
{
  "rows": [
    {
      "Username": "user1",
      "Rating": 2000,
      "GroupName": "student"
    }
  ],
  "total": 100
}
```

**前端使用**:
- [rankings.html](static/rankings.html): 加载排行榜数据

---

## 题目管理 API

### `/api/delete_problem` [POST]

**功能**: 删除题目。

**权限**: 需要 `EditPermission`

**输入** (JSON Body):
```json
{
  "problem_id": 1
}
```

**输出**:
```json
{ "status": "ok" }
```

---

### `/api/upload_problem` [POST]

**功能**: 上传新题目（tuack 格式 zip 包）。

**权限**: 需要 `EditPermission`

**输入**: Multipart Form
- `name` (string): 题目名称
- `title` (string): 题目标题
- `file` (file): zip 压缩包

**输出**:
```json
{
  "status": "ok",
  "problem_id": 10
}
```

**前端使用**:
- [upload_problem.html](static/upload_problem.html): 上传题目

---

### `/api/import_tuack` [POST]

**功能**: 导入 tuack 格式题目包。

**权限**: 需要 `EditPermission`

**输入**: Multipart Form
- `problem_id` (string): 题目 ID
- `file` (file): zip 压缩包

**输出**:
```json
{ "status": "ok" }
```

---

### `/api/import_data_zip` [POST]

**功能**: 导入题目测试数据 zip 包。

**权限**: 需要 `EditPermission`

**输入**: Multipart Form
- `problem_id` (string): 题目 ID
- `file` (file): zip 压缩包

**输出**:
```json
{ "status": "ok" }
```

---

## 编辑功能 API

### `/edit` [GET]

**功能**: 获取题目编辑页面。

**权限**: 需要 `EditPermission`

**输入**: 无

**输出**: HTML 页面 (`static/edit.html`)

---

### `/edit/modify` [POST]

**功能**: 修改题目描述（statement.md）。

**权限**: 需要 `EditPermission`

**输入** (JSON Body):
```json
{
  "problem_id": "1",
  "new_statement": "# 新题目描述\n..."
}
```

**输出**:
```json
{ "status": "ok" }
```

---

### `/edit/add_test` [POST]

**功能**: 添加测试点数据。

**权限**: 需要 `EditPermission`

**输入** (JSON Body):
```json
{
  "problem_id": "1",
  "test_index": 1,
  "input_data": "1 2\n",
  "output_data": "3\n"
}
```

**输出**:
```json
{ "status": "ok" }
```

---

## 页面路由汇总

以下路由直接返回 HTML 页面，不返回 JSON API：

| 路由 | 方法 | 输出页面 | 权限 |
|------|------|----------|------|
| `/problem` | GET | problem.html | 公开 |
| `/problemlist` | GET | problemlist.html | 公开 |
| `/contest` | GET | contest.html | 公开 |
| `/contest/{id}` | GET | contest_detail.html | 公开 |
| `/contest/{id}/leaderboard` | GET | contest_leaderboard.html | 公开 |
| `/contest_manage` | GET | contest_manage.html | - |
| `/ranking` | GET | rankings.html | 公开 |
| `/manage` | GET | manage.html | - |
| `/manage_users` | GET | manage_users.html | - |
| `/register` | GET | register.html | 公开 |
| `/create_user` | GET | create_user.html | - |
| `/create_group` | GET | create_group.html | - |
| `/upload_problem` | GET | upload_problem.html | - |
| `/edit` | GET | edit.html | - |
| `/submit` | GET | submit.html | 公开 |
| `/submissions` | GET | submissions.html | 公开 |
| `/submission/{id}` | GET | submission_detail.html | 公开 |
| `/user_profile` | GET | 重定向到 /user/{username} | 需要登录 |
| `/user/{username}` | GET | user_profile.html | 公开 |
| `/board` | GET | 无对应页面 | 公开 |
| `/postboard` | GET | postboard.html | - |

---

## 状态码说明

| 状态码 | 说明 |
|--------|------|
| 200 OK | 请求成功 |
| 400 Bad Request | 请求参数错误或缺少必要字段 |
| 401 Unauthorized | 未登录或会话失效 |
| 403 Forbidden | 无权限访问该资源 |
| 404 Not Found | 资源不存在 |
| 405 Method Not Allowed | HTTP 方法不支持 |
| 500 Internal Server Error | 服务器内部错误 |

---

## 特殊说明

1. **题目编号约定**: 前后端统一使用数字 `id` 表示题目编号，禁止使用 `name` 进行题目检索。

2. **时间格式**: 所有时间字段使用 ISO 8601 格式（RFC3339），例如 `2024-01-01T00:00:00Z`。

3. **文件上传**: 支持 multipart/form-data 格式，最大文件大小 100MB。

4. **分页约定**: 页码从 1 开始，每页数量默认 10，最大 100。

5. **密码生成**: 系统自动为新创建的用户生成 8 位强密码（字母数字混合）。
