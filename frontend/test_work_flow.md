# 网站自动化测试用例生成流程

深度优先搜索（DFS）执行说明

本文为执行者的操作指导说明，目标是在不预设业务知识的情况下，从网站入口自动发现功能、遍历所有可达路径，并基于遍历过程自动生成测试用例。执行过程中使用 Playwright MCP 执行操作，使用 Memory MCP 存储状态、使用 MCP-Shrimp Task Manager 管理任务列表，并使用 Sequential Thinking 控制执行逻辑。

---

## 一、目标说明

1. 从网站入口页开始自动遍历页面与功能。
2. 自动识别当前页面所有可执行操作（可点击、可输入、可提交、可切换等）。
3. 使用深度优先搜索（DFS）策略探索所有功能路径。
4. 遍历中发现新功能时将其加入待办任务列表继续深度探索。
5. 使用 Memory MCP 记录已访问页面状态以避免重复遍历。
6. 使用 MCP-Shrimp Task Manager 管理任务列表执行顺序。
7. 基于路径生成 Playwright 自动化测试用例与最终测试报告。

---

## 二、核心执行元素说明

| 名称               | 描述                                                       | 工具                                     |
| ------------------ | ---------------------------------------------------------- | ---------------------------------------- |
| 页面状态 State     | 页面 URL、结构摘要、上下文状态，用于判重和避免重复遍历     | Memory MCP                               |
| 操作 Action        | 由执行者在页面上触发的可执行动作（点击、输入、提交等）     | Playwright MCP + MCP-Shrimp Task Manager |
| 测试路径 Test Path | 按时间顺序排列的一系列 Action，形成完整的业务流或测试场景  | Sequential Thinking                      |
| 回溯               | 在一条测试路径结束或终止后，恢复到上一状态继续遍历其他分支 | DFS（深度优先搜索策略）                  |

---

## 三、工具协同方式

### Playwright MCP（执行网页操作）

职责：

- 控制浏览器实例（启动、关闭、上下文、页面）
- 执行 Action（点击、输入、选择、跳转、等待页面加载）
- 在执行 Action 后提供页面信息，用于构建新的 State
- 在回溯阶段，通过重放 Test Path 恢复至指定页面状态

产出：

- 执行结果（成功/失败/异常）
- 可用于断言的页面信息（URL、元素、文本、可见性等）

---

### Memory MCP（记录页面状态）

记录内容：

- URL
- 页面结构摘要（可交互元素、标题、关键区域信息）
- 上下文数据（登录状态、数据数量等）
- 状态唯一标识 StateID

用途：

- 判定新旧状态，避免重复遍历
- 为回溯恢复和路径复现提供参考记录

---

### MCP-Shrimp Task Manager（任务管理）

职责：

- 保存 Action 与其对应的 Test Path
- 管理 DFS 待办任务列表
- 栈结构：后进先出（LIFO）
- 在遍历中发现新 Action 时动态加入任务栈

用途：

- 控制 DFS 深度优先的执行顺序
- 保证每条分支独立且完整地被遍历

---

### Sequential Thinking（执行控制与决策）

职责：

- 明确当前状态、当前路径与待执行任务
- 识别可执行动作、评估执行可能产生的状态变化
- 规划下一步操作（继续深入或分支终结）

用途：

- 决定 Playwright MCP 的下一步执行内容
- 触发测试用例的生成和回溯阶段

## 四、DFS 遍历执行流程

### 初始化

1. 通过 Playwright MCP 启动浏览器环境并打开首页
2. 使用 Playwright MCP 获取页面信息，捕获初始状态（State_0），写入 Memory MCP
3. 使用 Playwright MCP 扫描页面可执行的 Action
4. 将扫描得到的所有 Action 加入 MCP-Shrimp 任务栈（LIFO）

---

### DFS 主循环（直到任务列表为空）

1. 从 MCP-Shrimp 任务栈弹出一个 Action（后进先出）
2. 根据该 Action 对应的 Test Path，使用 Playwright MCP 重放完整路径，恢复到执行前状态
3. 使用 Playwright MCP 执行该 Action，并等待页面变化稳定
4. 使用 Playwright MCP 捕获 Action 执行后的页面状态，构建 State_new
5. 将 State_new 提交给 Memory MCP，用于判定是否为新状态

#### 分支处理：

| 条件             | 处理方法                                                                                   |
| ---------------- | ------------------------------------------------------------------------------------------ |
| State_new 未记录 | 写入 Memory MCP；使用 Playwright MCP 识别新的 Action，将其加入 MCP-Shrimp 任务队列继续深入 |
| State_new 已记录 | 视为路径结束：记录当前 Test Path 作为可生成测试用例的完整路径                              |

6. 执行回溯：使用 Sequential Thinking 判断回溯点，再由 Playwright MCP 重放对应路径恢复到上一状态
7. 从 MCP-Shrimp 任务栈获取下一个 Action，继续 DFS 循环

---

## 五、测试用例生成策略

当一条测试路径完成或到达已访问过的重复状态时，生成对应的测试用例。测试用例基于该 Test Path 中的 Action 顺序构建，并结合 Playwright MCP 的可执行逻辑。

测试用例内容包含：

- 用例标题
- 前置条件（如登录角色、环境配置）
- 操作步骤（完整路径中 Action 的顺序描述）
- 期望行为与验证点（基于 Playwright MCP 可验证内容）

转换规则：

- Test Path 中每个 Action 转换为 Playwright 中可执行的一步操作说明
- 关键页面变化记录为断言点，例如：URL、DOM 结构、关键文本、元素可见性
- 当路径终止或遇到重复状态时认为该路径完成，用于生成完整用例

---

## 六、流程结构示例（概念）

入口页面
↓
发现全部 Action → 加入 MCP-Shrimp 任务栈
↓
弹出 Action → 通过 Playwright MCP 深入执行
↓
产生新状态？
↓ ↓
是 否
记录并扩展 生成基于 Test Path 的测试用例
↓
使用 Playwright MCP 回放路径回溯上一状态
↓
任务栈是否为空？
↓ ↓
是 否 → 继续执行
遍历完成

---

# 七、交付物 Deliverables

## 1. Playwright 自动化测试用例

基于 DFS 遍历过程中记录的完整 Test Path 自动生成 Playwright 测试用例。

内容要求：

- 由 DFS 路径转化为 Playwright 测试逻辑，可稳定执行
- 每条路径生成一个独立用例文件或一个 describe 下的 it 块
- 包含：
  - 基础环境初始化（浏览器、上下文与页面）
  - 按 Test Path 顺序执行 Action
  - 使用 Playwright MCP 生成断言，包括：
    - URL 变化校验
    - 页面标题变化校验
    - 页面关键元素可见性或属性校验
    - 提示信息或状态变化校验
- 文件命名规范：
  `TC_001_Keyword_Sequence.test.ts`

结构示例（文本，不含代码）：

TestCase: TC_001  
Title: 用户从首页进入订单列表并创建订单  
Steps:

1. 打开首页
2. 点击“订单管理”
3. 点击“新建订单”  
   Assertions:  
    页面跳转到 /order/create  
    显示创建成功提示
