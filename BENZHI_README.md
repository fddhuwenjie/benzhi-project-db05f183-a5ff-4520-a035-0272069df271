# BENZHI_README

## 项目说明
- 项目：benzhi-project-db05f183-a5ff-4520-a035-0272069df271
- 项目用途：已完整实现方言语音语料批次质量放行工作台，覆盖规范冻结、双人独立标注、字段级分歧裁决、确定性抽审、限定返修、定向复审、不可变发布清单及摘要重算验证。
- Go 工具链：`golang:1.22`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 项目描述
- 项目名称：DialectCorpusReleaseGate
- 项目介绍：面向方言语音资料整理团队的语料标注批次质量放行工作台。系统只处理一条完整流程：批次建档并冻结标注规范，登记语音条目并完成双人独立标注，对分歧进行裁决，执行抽样审计与定向返修，最终冻结可验证的发布清单。项目按 standard 档规划至少 2200 行真实生产 Go 代码和至少 20 个生产 Go 文件。
- 项目概述：面向方言语音资料整理团队的语料标注批次质量放行工作台。系统只处理一条完整流程：批次建档并冻结标注规范，登记语音条目并完成双人独立标注，对分歧进行裁决，执行抽样审计与定向返修，最终冻结可验证的发布清单。项目按 standard 档规划至少 2200 行真实生产 Go 代码和至少 20 个生产 Go 文件。
- 核心工作流：单个语料批次从 DRAFT 建档开始，负责人冻结标签集、转写规则和最低一致率后进入 ANNOTATING；每条语音资料完成两份相互隔离的标注后计算一致性并进入 ADJUDICATING；分歧全部裁决后进入 AUDITING，审计不通过则转为 CORRECTION 并仅返修命中范围，复审通过后进入 RELEASED，生成不可变发布清单及摘要验证结果。
- 对外接口：由 Go 服务提供一个原生 HTML、CSS、JavaScript 浏览器工作台及仅供该页面使用的同源 JSON 端点；页面在单个批次视图中完成建档、标注、裁决、抽审、返修和发布验证，不引入 Node 构建链。服务支持 -addr=127.0.0.1:<port>，也可读取 PORT 并绑定 127.0.0.1:<PORT>，默认监听 127.0.0.1:19081，禁止默认绑定 8080、80、3000、0.0.0.0。

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...

cd /app && GOTOOLCHAIN=local go run ./cmd/server -selftest -selftest-timeout=12s -addr=127.0.0.1:19081

cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh

./build_benzhi_docker.sh benzhi-project-db05f183-a5ff-4520-a035-0272069df271-amd64 linux/amd64

./build_benzhi_docker.sh benzhi-project-db05f183-a5ff-4520-a035-0272069df271-arm64 linux/arm64

docker run -it benzhi-project-db05f183-a5ff-4520-a035-0272069df271-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -selftest -selftest-timeout=12s -addr=127.0.0.1:19081`
