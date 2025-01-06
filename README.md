# AI大模型数据综合处理系统 (ALMDCPS)

> AI Large Model Data Comprehensive Processing System

## 项目概述

ALMDCPS 是一个基于 Go 语言和 Gin 框架开发的企业级数据处理系统。该系统集成了用户管理、数据处理、文件管理等多个模块，旨在为企业提供高效、安全、可靠的数据处理解决方案。

### 核心特性

- **用户管理系统**：完整的用户认证和授权机制
- **文件处理引擎**：支持多种格式文件的批量处理
- **数据分析平台**：提供数据可视化和分析功能
- **实时进度监控**：处理过程实时反馈
- **安全性保障**：数据加密存储，访问权限控制

## 技术架构

### 后端技术栈
- **框架**：Gin Web Framework
- **数据库**：MySQL
- **缓存**：Redis（可选）
- **会话管理**：Gin-Sessions
- **文件处理**：自研引擎

### 前端技术栈
- **框架**：原生 JavaScript
- **UI 框架**：Bootstrap 5
- **HTTP 客户端**：Fetch API
- **样式处理**：CSS3

## 功能模块

### 1. 用户管理系统
- **用户认证**
  - 账号密码登录
  - 会话管理
  - 权限控制
- **用户信息**
  - 个人资料管理
  - 密码修改
  - 操作日志

### 2. 文件处理系统
- **文件上传**
  - 支持 .xlsx 格式
  - 文件大小限制
  - 类型验证
- **批量处理**
  - 自动化处理流程
  - 错误处理机制
  - 结果导出
- **进度监控**
  - 实时进度显示
  - 状态查询
  - 错误提示

### 3. 数据分析平台
- **文件重命名工具**
  - 批量重命名
  - 自定义规则
  - 操作日志
- **数据分析服务**（开发中）
  - 数据可视化
  - 报表生成
  - 趋势分析

## 项目结构

```
ALMDCPS/
├── api/          # API 接口定义
├── config/       # 配置文件
├── models/       # 数据模型
├── utils/        # 工具函数
├── web/          # 前端资源
│   ├── css/      # 样式文件
│   ├── js/       # JavaScript 文件
│   └── images/   # 图片资源
├── chengshi/     # 工具集
└── main.go       # 主程序入口
```

## 系统流程

```
+-------------------+     +-------------------+     +-------------------+
|    用户访问首页    | --> |    用户身份验证    | --> |    功能模块选择    |
+-------------------+     +-------------------+     +-------------------+
                                                           |
                                                           v
+-------------------+     +-------------------+     +-------------------+
|    返回处理结果    | <-- |    数据处理流程    | <-- |    文件上传处理    |
+-------------------+     +-------------------+     +-------------------+
```

## 部署要求

### 系统要求
- Go 1.16+
- MySQL 5.7+
- 现代浏览器（Chrome、Firefox、Safari、Edge）

### 环境配置
1. 安装 Go 环境
2. 配置 MySQL 数据库
3. 设置环境变量
4. 安装依赖包

## 快速开始

1. **克隆项目**
   ```bash
   git clone https://github.com/your-repo/ALMDCPS.git
   cd ALMDCPS
   ```

2. **安装依赖**
   ```bash
   go mod download
   ```

3. **配置数据库**
   - 创建数据库
   - 修改 config/database.go 配置

4. **启动服务**
   ```bash
   go run main.go
   ```

5. **访问系统**
   - 打开浏览器访问：http://localhost:8081

## 使用指南

### 用户登录
1. 访问系统首页
2. 点击"登录"按钮
3. 输入用户名和密码
4. 提交登录请求

### 文件处理
1. 登录系统
2. 选择要处理的文件
3. 上传文件
4. 等待处理完成
5. 下载处理结果

## 开发计划

### 当前版本 (v1.0.0)
- [x] 用户认证系统
- [x] 文件上传功能
- [x] 文件重命名工具
- [ ] 数据分析平台

### 下一版本 (v1.1.0)
- [ ] 数据可视化功能
- [ ] 批量处理优化
- [ ] 用户权限管理
- [ ] 性能优化

## 技术支持

如遇到问题，请联系：
- 邮箱：support@duanmu.com
- 官网：www.duanmu.com

## 许可说明

版权所有 2024 端木科技
保留所有权利
