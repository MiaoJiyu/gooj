# 配置文件说明

## 概述

Gooj 系统现在支持通过配置文件来管理数据库类型和连接信息、服务器端口等设置。配置文件采用 YAML 格式，位于 `config/config.yaml`。

## 配置文件结构

```yaml
# Database configuration
database:
  type: "sqlite"   # sqlite | mysql
  
  # SQLite configuration
  sqlite:
    path: "data/app.db"
    
  # MySQL configuration
  mysql:
    host: "localhost"
    port: 3306
    user: "root"
    password: ""
    dbname: "gooj"

# Server configuration
server:
  port: 8081

# Command service configuration
cmd:
  port: 9090
  host: "127.0.0.1"
```

## 配置项说明

### database
- **type**: 数据库类型，支持 `sqlite` 或 `mysql`
- **sqlite**: SQLite 特定配置
  - **path**: SQLite 数据库文件路径
- **mysql**: MySQL 特定配置
  - **host**: MySQL 服务器地址
  - **port**: MySQL 服务器端口
  - **user**: 用户名
  - **password**: 密码
  - **dbname**: 数据库名称

### server
- **port**: HTTP 服务器监听端口
  - 默认值：`8081`

### cmd
- **host**: 命令服务监听地址
  - 默认值：`127.0.0.1`
- **port**: 命令服务监听端口
  - 默认值：`9090`

### services
- **sql**: 是否启动数据库服务 (true/false)
  - 默认值：`true`
- **judge**: 是否启动判题服务 (true/false)
  - 默认值：`true`
- **file**: 是否启动文件服务 (true/false)
  - 默认值：`true`

## 启动时指定配置文件

启动服务器时，可以使用 `-config` 参数指定配置文件路径：

```bash
# 使用默认配置文件 (config/config.yaml)
./gooj -method run

# 使用自定义配置文件
./gooj -method run -config /path/to/your/config.yaml
```

## 默认值

如果配置文件不存在或加载失败，系统将使用以下默认值：
- 数据库类型：`sqlite`
- SQLite 路径：`data/app.db`
- MySQL 配置：localhost:3306, user=root, no password, db=gooj
- 服务器端口：`8081`
- 命令服务地址：`127.0.0.1:9090`
- 服务开关：所有服务默认开启 (true)

## 示例

### SQLite 开发环境配置
```yaml
database:
  type: "sqlite"
  sqlite:
    path: "data/dev.db"

server:
  port: 8080

cmd:
  port: 9091
  host: "127.0.0.1"
```

### MySQL 生产环境配置
```yaml
database:
  type: "mysql"
  mysql:
    host: "db.example.com"
    port: 3306
    user: "gooj_user"
    password: "secure_password"
    dbname: "gooj_prod"

server:
  port: 80

cmd:
  port: 9090
  host: "127.0.0.1"

services:
  sql: true
  judge: true
  file: false  # Disable file service
```

### 最小化服务配置（仅 Web 界面）
```yaml
database:
  type: "sqlite"
  sqlite:
    path: "data/app.db"

server:
  port: 8080

cmd:
  port: 9090
  host: "127.0.0.1"

services:
  sql: true
  judge: false  # Disable judge service
  file: false   # Disable file service
```

## 注意事项

1. 使用 MySQL 时，请确保已创建对应的数据库
2. 配置文件中的 `type` 字段必须与对应的配置块匹配
3. 切换数据库类型时，需要重新运行数据库迁移
4. 服务开关可以控制启动时是否启动对应服务，便于调试和资源优化
5. 如果禁用 SQL 服务，数据库初始化将不会执行
6. 如果禁用 Judge 服务，判题功能将不可用
7. 如果禁用 File 服务，文件上传和下载功能将不可用
