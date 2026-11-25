import type { Locale } from './i18n';

// Chrome shared by every page that is not a component page. The component pages
// keep their own dictionary in i18n.ts; both are typed so a missing translation
// is a compile error and a test failure rather than mixed-language prose.
const english = {
  skip: 'Skip to content', home: 'Viva home',
  navDocs: 'Docs', navComponents: 'Components', navShowcase: 'Showcase', navApi: 'API',
  primaryNavigation: 'Primary', language: 'Documentation language',
  repository: 'Viva on GitHub',
  search: 'Search',
  searchPlaceholder: 'Search components, guides and API…',
  searchEmpty: 'No matches. Try another name or category.',
  footerTagline: 'A Go-native desktop application framework and runtime.',
  footerResources: 'Resources',
  footerStart: 'Getting started',
  footerCatalog: 'Component catalog',
  footerGuides: 'Guides',
  footerApi: 'API reference',
  footerRepo: 'GitHub repository',
  footerAgents: 'Coding agent skill',
  footerPlatforms: 'Platforms',
  footerLicense: 'Apache-2.0 license',
  footerBuilt: 'Built by the repository generator: system fonts, no third-party scripts.',
  onThisPage: 'On this page',
  skillHeading: 'The skill in full', skillNote: 'The same text as the file you install, so an agent that does not load skills can read it here or have it pasted into its instructions file, such as AGENTS.md or CLAUDE.md.', skillCopy: 'Copy the skill', skillRaw: 'Raw file',
  groupStart: 'Start', groupConcepts: 'Concepts', groupGuides: 'Guides', groupDistribute: 'Build and ship', groupMore: 'More',
  docsTitle: 'Documentation', docsSummary: 'Guides for building desktop applications with Viva: the programming model, components, packaging, and what each platform supports today.',
  docsIntro: 'The guides cover the programming model, declarative layout, input and text, overlays, system integration, packaging and the measurements behind the size and idle-cost budgets. Every benchmark the site quotes names the machine and the build that produced it, so you can rerun it.',
  apiTitle: 'API reference', apiSummary: 'Every exported type, function and constant of the Viva packages, generated from the Go source and its doc comments.',
  apiPackages: 'Packages', apiType: 'Type', apiPackage: 'Package', apiTypes: 'Types', apiFunctions: 'Functions', apiConstants: 'Constants', apiVariables: 'Variables',
  apiConstructors: 'Constructors', apiMethods: 'Methods', apiOverview: 'Overview', apiImport: 'Import path',
  apiTable: 'API', apiField: 'Field', apiDescription: 'Description', apiSignature: 'Signature',
  apiPageGroup: 'Page API', apiCanvasGroup: 'Canvas recipe',
  apiCanvasNote: 'Use inside ui.Control/Draw, or copy the recipe with viva add. Prefer the ui.* page control where one exists.',
  apiEnglishOnly: 'API documentation is generated from the Go source and is in English.',
  notFoundGuide: 'This guide has no translation yet.',
};

const chinese: typeof english = {
  skip: '跳转到正文', home: 'Viva 首页',
  navDocs: '文档', navComponents: '组件', navShowcase: '案例', navApi: 'API',
  primaryNavigation: '主导航', language: '文档语言',
  repository: 'Viva 的 GitHub 仓库',
  search: '搜索',
  searchPlaceholder: '搜索组件、指南与 API…',
  searchEmpty: '没有匹配项，换个名称或分类试试。',
  footerTagline: 'Go 原生的桌面应用框架与运行时。',
  footerResources: '资源',
  footerStart: '开始使用',
  footerCatalog: '组件目录',
  footerGuides: '指南',
  footerApi: 'API 参考',
  footerRepo: 'GitHub 仓库',
  footerAgents: 'AI 编程助手 skill',
  footerPlatforms: '平台支持',
  footerLicense: 'Apache-2.0 许可证',
  footerBuilt: '由仓库里的生成器构建：系统字体，无第三方脚本。',
  onThisPage: '本页内容',
  skillHeading: '完整的 skill 内容', skillNote: '与安装的文件内容相同（英文原文），所以不加载 skill 的代理可以直接在这里阅读，也可以把它粘贴进自己的指令文件，例如 AGENTS.md 或 CLAUDE.md。', skillCopy: '复制 skill', skillRaw: '原始文件',
  groupStart: '开始', groupConcepts: '概念', groupGuides: '指南', groupDistribute: '构建与发布', groupMore: '更多',
  docsTitle: '文档', docsSummary: '用 Viva 构建桌面应用的指南：编程模型、组件、打包，以及各平台目前实际支持什么。',
  docsIntro: '这些指南覆盖编程模型、声明式布局、输入与文本、浮层、系统集成、打包，以及体积与空闲开销预算背后的实测。站点引用的每个基准都写明了机器和构建方式，你可以自己重跑。',
  apiTitle: 'API 参考', apiSummary: 'Viva 各个包导出的全部类型、函数与常量，由 Go 源码及其文档注释生成。',
  apiPackages: '包', apiType: '类型', apiPackage: '包', apiTypes: '类型', apiFunctions: '函数', apiConstants: '常量', apiVariables: '变量',
  apiConstructors: '构造函数', apiMethods: '方法', apiOverview: '概览', apiImport: '导入路径',
  apiTable: 'API', apiField: '字段', apiDescription: '说明', apiSignature: '签名',
  apiPageGroup: '页面 API', apiCanvasGroup: '画布配方',
  apiCanvasNote: '在 ui.Control/Draw 内使用，或用 viva add 复制源码；有页面控件时优先用 ui.*。',
  apiEnglishOnly: 'API 文档由 Go 源码生成，内容为英文。',
  notFoundGuide: '这篇指南暂无翻译。',
};

export const site = { zh: chinese, en: english } as const;
export function siteText(locale: Locale): typeof english {
  return site[locale];
}
