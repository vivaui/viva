---
title: 用 AI 编程助手构建应用
summary: 安装 Viva skill，让 AI 编程助手遵守框架的规则，并在不打开窗口的情况下检查界面。
order: 16
group: start
skill: building-with-viva
---

Viva 自己绘制界面，只在被请求时重绘，也没有布局缓存，所以按网页或保留式工具包的写法来写，产出的代码会被评审拒绝。`building-with-viva` skill 是一份简短的说明，Claude Code、Codex 这类 AI 编程助手在改动 Viva 应用之前加载它。它只是一个 Markdown 文件，没有需要运行的代码。

## skill 涵盖什么

- 如何判断一个需求属于应用行为，还是应当放进 Viva 而不是应用里的可复用桌面能力。
- 评审会拒绝的规则：绘制路径中不用反射或属性包，空闲时不做任何工作，每次绘制不分配内存，不手工给命中目标编号，不做未经测量的性能声明。
- 一个界面如何编写：`gfx.Tree` 中的标识，在声明控件的地方处理点击，用 `ui` 布局。
- 如何在不打开窗口的情况下验证改动，顺序见[无需窗口的测试](/zh/docs/testing/)，以及对没能检查的部分该如何如实说明。
- 指南、示例和测量数据在哪里。

它指向各篇指南而不重复其内容，所以足够短，可以整篇读完，也不会因为指南变化而过时。完整内容就在本页下方。

## 安装

把这个文件下载到项目的 skills 目录。Claude Code 读取 `.claude/skills`；遵循同一约定的其他 AI 编程助手读取 `.agents/skills`。

```sh
mkdir -p .claude/skills/building-with-viva
curl -fsSL https://vivaui.dev/skills/building-with-viva/SKILL.md \
  -o .claude/skills/building-with-viva/SKILL.md
```

在 Viva 自己的仓库副本中，请改为复制或软链 `docs/skills/building-with-viva`。在那里，仓库的 `AGENTS.md` 优先于这个 skill。

如果你的 AI 编程助手不加载 skill，就把本页下方的完整内容复制进它读取的指令文件，例如 `AGENTS.md` 或 `CLAUDE.md`，或者直接让它阅读这个页面。

## 保持最新

网站提供的就是仓库 `docs/skills` 中的那个文件，每次构建站点时都会重新生成，所以下载到的内容、本页展示的内容与旁边的指南始终一致。升级 Viva 之后再运行一次同样的命令，并把文件随项目一起提交，这样参与项目的人和 AI 助手读到的是同一份说明。想了解这些规则背后的原因，请阅读[编程模型](/zh/docs/programming-model/)指南。
