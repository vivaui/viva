import type { Page, Section } from './types';

export const locales = ['zh', 'en'] as const;
export type Locale = typeof locales[number];
export const defaultLocale: Locale = 'zh';
export const languageNames = { zh: '简体中文', en: 'English' };

export function isLocale(value: unknown): value is Locale {
  return value === 'zh' || value === 'en';
}

export function componentsPath(locale: Locale): string {
  return `/${locale}/components/`;
}

export function componentPath(locale: Locale, component?: string): string {
  return component ? `/${locale}/ui/${component}/` : componentsPath(locale);
}

export function languagePath(locale: Locale, currentID: string): string {
  return currentID === 'components' ? componentsPath(locale) : componentPath(locale, currentID);
}

const english = {
  skip: 'Skip to content', home: 'Viva documentation home',
  components: 'Components', componentsSummary: 'All components, grouped by category.',
  language: 'Documentation language', darkTheme: 'Dark theme', light: 'Light', dark: 'Dark',
  browse: 'Browse components', search: 'Search components', searchPlaceholder: 'Search…',
  noMatches: 'No matching components. Try another name or category.', matches: 'matching components',
  preview: 'Preview', copySource: 'Copy source',
  exampleUsage: 'Usage', copyUsage: 'Copy usage',
  copyInstall: 'Copy install command', copyCode: 'Copy code', copying: 'Copying…', copied: 'Copied',
  copiedNotice: 'Copied to clipboard.', copyFailed: 'Copy failed. Select the text and press Ctrl+C or ⌘C.',
  reset: 'Reset', resetExample: 'Reset this example', openPreview: 'Open preview', retry: 'Retry preview',
  starting: 'Starting Go WebAssembly…', waiting: 'Preview loads as you scroll into view.',
  noJS: 'Enable JavaScript to run the preview. Go source and install commands remain available.',
  unavailable: 'Preview unavailable',
  timeout: 'The runtime did not respond within 60 seconds. Retry the preview.',
  runtimeError: 'The Go runtime reported an error:',
  installation: 'Installation', installHint: 'Copy the recipe into your own project.', installUnavailable: 'The installer does not package this recipe yet. Use the source link above and keep its package dependencies together.',
  installCommand: 'Install command', repositoryCommand: 'Repository command',
  viewSource: 'View source',
  toc: 'On this page', keyboard: 'Keyboard', keyboardIntro: 'Use Tab to move between controls and Ctrl/⌘ K to search the catalog. Inside a preview, the Go runtime handles keyboard input.',
};

const chinese: typeof english = {
  skip: '跳转到正文', home: 'Viva 文档首页',
  components: '组件', componentsSummary: '全部组件，按分类浏览。',
  language: '文档语言', darkTheme: '深色主题', light: '浅色', dark: '深色',
  browse: '浏览组件', search: '搜索组件', searchPlaceholder: '搜索组件…',
  noMatches: '没有匹配的组件，试试名称或分类。', matches: '个匹配组件',
  preview: '预览', copySource: '复制源码',
  exampleUsage: '用法', copyUsage: '复制用法',
  copyInstall: '复制安装命令', copyCode: '复制代码', copying: '复制中…', copied: '已复制',
  copiedNotice: '已复制到剪贴板。', copyFailed: '复制失败，请选中文本后按 Ctrl+C 或 ⌘C。',
  reset: '重置', resetExample: '重置此示例', openPreview: '单独打开', retry: '重试预览',
  starting: '正在启动 Go WebAssembly…', waiting: '滚动到此处时加载预览。',
  noJS: '启用 JavaScript 后可运行预览；源码和安装命令仍可直接查看。',
  unavailable: '预览暂不可用',
  timeout: '运行时在 60 秒内未响应，请重试预览。',
  runtimeError: 'Go 运行时报错：',
  installation: '安装', installHint: '将组件源码复制到自己的项目，直接修改。', installUnavailable: '安装器尚未打包此配方。请使用标题旁的源码链接，并一并保留它依赖的同包组件。',
  installCommand: '安装命令', repositoryCommand: '仓库命令',
  viewSource: '查看源码',
  toc: '本页内容', keyboard: '键盘操作', keyboardIntro: 'Tab 在控件间移动；Ctrl/⌘ K 搜索组件。进入预览后，键盘事件由 Go 运行时处理。',
};

export const messages = { zh: chinese, en: english };
const categories: Record<string, string> = {
  Actions: '操作', Communication: '沟通', Data: '数据', 'Data Display': '数据展示',
  'Date & Time': '日期与时间', Disclosure: '展开收起', Display: '展示', Feedback: '反馈',
  Forms: '表单', Layout: '布局', Media: '媒体', Navigation: '导航', Overlay: '浮层', Overlays: '浮层',
};
export function categoryLabel(locale: Locale, value: string): string {
  if (!categories[value]) throw new Error(`Missing category translation: ${value}`);
  return locale === 'zh' ? categories[value] : value;
}

const chineseComponentNames: Record<string, string> = {
  button: '按钮',
  'button-group': '按钮组',
  toggle: '切换按钮',
  'toggle-group': '切换按钮组',
  bubble: '气泡',
  message: '消息',
  'message-scroller': '消息滚动',
  table: '表格',
  tree: '树',
  attachment: '附件',
  avatar: '头像',
  carousel: '走马灯',
  chart: '图表',
  'metric-card': '指标卡',
  calendar: '日历',
  accordion: '手风琴',
  collapsible: '折叠面板',
  badge: '徽章',
  item: '列表项',
  kbd: '键帽',
  markdown: 'Markdown 文本',
  marker: '标记',
  typography: '排版',
  alert: '警告',
  empty: '空状态',
  progress: '进度条',
  skeleton: '骨架屏',
  sonner: '堆叠通知',
  spinner: '加载中',
  toast: '轻提示',
  checkbox: '复选框',
  combobox: '组合框',
  field: '表单字段',
  form: '表单',
  input: '输入框',
  'input-group': '输入框组',
  'input-otp': '验证码输入',
  label: '标签',
  'native-select': '原生选择器',
  questionnaire: '问卷',
  'radio-group': '单选组',
  'segmented-control': '分段控制器',
  select: '选择器',
  slider: '滑块',
  switch: '开关',
  textarea: '多行输入',
  'aspect-ratio': '宽高比',
  card: '卡片',
  resizable: '可调整大小',
  'scroll-area': '滚动区域',
  separator: '分隔线',
  breadcrumb: '面包屑',
  menubar: '菜单栏',
  'navigation-menu': '导航菜单',
  pagination: '分页',
  sidebar: '侧边栏',
  tabs: '标签页',
  'alert-dialog': '警告对话框',
  popover: '弹出框',
  sheet: '侧边面板',
  tooltip: '工具提示',
  command: '命令面板',
  'context-menu': '上下文菜单',
  dialog: '对话框',
  drawer: '抽屉',
  'dropdown-menu': '下拉菜单',
  'hover-card': '悬停卡片',
  'code-diff': '代码差异',
  'split-button': '分离按钮',
};

export function componentName(locale: Locale, id: string, fallback: string): string {
  return locale === 'zh' ? chineseComponentNames[id] ?? fallback : fallback;
}

export function componentLabel(locale: Locale, id: string, fallback: string): string {
  const name = componentName(locale, id, fallback);
  return locale === 'zh' && name !== fallback ? `${fallback} ${name}` : fallback;
}

const englishComponents: Record<string, { summary: string; examples: Record<string, [string, string]> }> = {
  button: { summary: 'Button variants, disabled states and caller-owned click counts.', examples: {
    default: ['Click counter', 'Click or use the keyboard to update an application-owned counter.'],
    variants: ['Six variants', 'Production button variants share the same click counter.'],
    disabled: ['Disabled variants', 'Disabled controls keep their semantics and cannot activate.'],
  } },
  badge: { summary: 'Status labels that size naturally to their content.', examples: {
    default: ['Content-sized', 'Short labels measure their own width instead of filling a column.'],
    variants: ['Six variants', 'Default, secondary, destructive, outline, ghost and link presentations.'],
    'long-text': ['Long labels', 'Mixed Chinese and English labels wrap to the next row when needed.'],
  } },
  checkbox: { summary: 'Controlled selections, grouping and disabled states.', examples: {
    default: ['Controlled checkbox', 'Click or press Space to change the application-owned selection.'],
    selection: ['Multiple selections', 'Each checkbox has a stable identity and an independent selected value.'],
    states: ['Validation and disabled states', 'The required selection shows validation feedback alongside disabled unchecked and checked controls.'],
  } },
  switch: { summary: 'Controlled settings with explicit state transitions.', examples: {
    default: ['Controlled switch', 'Toggle a setting without hidden component-owned state.'],
    sizes: ['Two sizes', 'Compare the default and compact switch geometry.'],
    states: ['Disabled states', 'Disabled switches stay visible but do not change state.'],
  } },
  input: { summary: 'Real native editing, with an explicitly visual-only Web preview.', examples: {
    default: ['Native editing', 'The native provider connects an Editor for text, selection and IME.'],
    placeholder: ['Empty field', 'Empty values show a placeholder; native focus provides the caret.'],
    invalid: ['Validation state', 'Demo rule: any nonempty value containing @ clears the error.'],
    states: ['Disabled and read-only', 'Fixed examples never expose an editor to the native provider.'],
  } },
  dialog: { summary: 'Open, confirm and dismiss with a real modal focus scope.', examples: {
    default: ['Open a dialog', 'Open from the trigger; confirm, cancel, close or press Escape to dismiss.'],
    confirm: ['Confirmation', 'Starts open. Tab remains in the dialog; reopen it after closing.'],
    destructive: ['Destructive action', 'Only updates local demo state. No files or other data are deleted.'],
  } },
  'code-diff': { summary: 'An aligned two-pane comparison with native editing on the right.', examples: {
    default: ['Two-pane diff', 'Green marks local additions and red marks removed server content while both panes keep aligned line numbers.'],
  } },
  'split-button': { summary: 'Independent primary and menu actions in one joined control.', examples: {
    default: ['Save and more actions', 'The left target performs the primary action; the right target toggles the caller-owned menu state.'],
  } },
};

const chineseSections: Record<string, Record<string, Omit<Section, 'ID'>>> = {
  button: {
    usage: { Title: '用法', Text: '示例用 flex.Frame.Button 声明按钮；Clicked 在同一条声明里返回点击或键盘激活。重绘时保持 ID 稳定，状态由应用持有。', Items: ['Default、Secondary、Outline、Ghost、Destructive、Link 共用同一个交互模型。', '按钮默认按标签测量宽度；需要固定尺寸时用 Width。', 'XSmall、Small、Large 与图标尺寸由显式选项指定；纯图标按钮也需要可访问名称。'] },
    keyboard: { Title: '键盘与状态', Text: 'Tab 移动焦点，Enter 或空格激活可用按钮。鼠标获得焦点时不显示键盘焦点环。', Items: ['禁用控件不能激活，也不参与焦点遍历。', '悬停、按下和焦点状态直接读取 Canvas，不重复保存在应用状态里。'] },
  },
  badge: {
    usage: { Title: '默认按内容确定宽度', Text: 'PaintBadge 测量文本并返回实际边界。传入起点即可，无需让徽章撑满容器。', Items: ['Default 用于强调，Secondary 更弱，Outline 使用描边，Destructive 表示错误状态。', '长文本示例包含中英文混排；各平台的系统字体度量可能不同。'] },
    keyboard: { Title: '语义', Text: '徽章是标签，不是按钮；不接受焦点，也不创建点击目标。需要执行操作时请使用按钮。', Items: [] },
  },
  checkbox: {
    usage: { Title: '受控选择', Text: '应用持有 checked 布尔值。BeginCheckbox 读取它、声明复选框语义并绘制当前状态；点击回调负责改变状态。', Items: ['每个复选框都需要稳定 ID 和明确标签。', 'Disabled 与 Invalid 是显示和交互选项，不管理校验业务状态。', '当前 API 未实现不确定态，示例不会伪造该能力。'] },
    keyboard: { Title: '键盘与可访问性', Text: 'Tab 聚焦可用复选框；空格或 Enter 与点击使用同一个状态回调。选中值通过语义树暴露。', Items: [] },
  },
  switch: {
    usage: { Title: '明确的开关状态', Text: 'BeginSwitch 读取应用持有的布尔值。Label 命名控件，Small 改变紧凑尺寸，不创建不同的状态模型。', Items: ['开关适合立即生效的设置，不应用来确认操作。', '禁用开关仍然可见，但不会改变状态。', '浏览器展示最终状态；原生过渡时序应在原生示例中验证。'] },
    keyboard: { Title: '键盘与状态', Text: 'Tab 聚焦可用开关，Enter 或空格通过点击处理器切换状态。颜色由静态类型的主题 token 决定。', Items: [] },
  },
  input: {
		usage: { Title: '文本由应用持有', Text: 'PaintInput 绘制输入快照与命中区域。Web 与原生文本桥接连接同一个由调用方持有的 Go Editor；Editor 保存文本、选区与组合输入，各平台只负责传递文本系统事件。', Items: ['仅在字段真正拥有平台编辑焦点时隐藏静态文字。', '失焦时，在关闭平台编辑器的同一帧将当前文本重新绘制为静态标签。', 'InputSnapshot 的选区使用 UTF-8 字节偏移，平台文本事件使用 rune 索引，两者不可混用。'] },
		keyboard: { Title: '编辑与限制', Text: 'Web 与原生示例通过同一个 Go Editor 处理插入、退格、删除、左右移动、Shift 扩选与组合输入。macOS 还支持点击定位、拖动选区和 Command-A。', Items: ['单行输入中 Tab 用于切换焦点，不插入制表符。', '只读和禁用示例不启用平台编辑器。', '浏览器与桌面桥接仅事件传输方式不同，应用文本状态始终由同一份 Go 状态持有。', '各平台的剪贴板快捷键、撤销重做和多行编辑需要分别验证。'] },
  },
  dialog: {
    usage: { Title: '真实模态焦点范围', Text: 'BeginDialog 绘制遮罩并保持图层和焦点范围有效；绘制内容，用 Actions 排布页脚按钮（每个按钮宽度只按自己的文字，按给定顺序靠右排列），再调用 End。应用持有 open 状态与各操作 ID。', Items: ['触发、关闭、取消、确认使用不同的稳定 ID。', '打开后限制键盘焦点遍历；关闭时尽可能恢复原先焦点。', '本示例不通过点击遮罩关闭，请使用按钮或 Escape。确认与危险操作示例默认打开。'] },
    keyboard: { Title: '打开、确认与关闭', Text: '打开后使用 Tab 遍历操作，Escape 取消，或点击 Confirm / Cancel。结果只更新预览，不执行外部操作。', Items: [] },
  },
  'code-diff': {
    usage: { Title: '行对齐的源码对比', Text: 'CodeDiff 根据调用方持有的 CodeDiffResult 绘制两栏并同步滚动。左栏是固定基准版本，右栏连接与 CodeEditor、Textarea 相同的平台文本编辑器。', Items: ['用 DiffCodeLines 生成新增、删除与修改行，不改变任一源码字符串。', '长行共享横向偏移，两栏共享纵向偏移。', '新增和删除行除背景色外，还提供符号与行号提示。'] },
    keyboard: { Title: '编辑与滚动', Text: '右栏保留基于字节偏移的选区、组合输入与插入点位置，左栏保持只读对齐。', Items: ['鼠标滚轮和滚动条拖动会更新共享偏移。', '只读预览不向平台暴露编辑器。', '应用应合并高频重算并丢弃较旧文本产生的结果。'] },
  },
  'split-button': {
    usage: { Title: '两个独立操作', Text: 'SplitButton 用一个相连外观绘制主操作与菜单触发区，同时声明两个稳定 ID。调用 Paint 后通过 Clicked 分别读取结果。', Items: ['主操作与菜单两半具有独立的悬停、焦点、按下和禁用语义。', 'Open 由调用方持有，并作为菜单触发区的展开状态暴露。', '当菜单用于选择一个主要结果的不同方式时使用 SplitButton。'] },
    keyboard: { Title: '键盘与可访问性', Text: 'Tab 分别到达每个可用半区；Enter 与空格通过和鼠标相同的命中路径激活当前焦点。', Items: ['没有可见文字的菜单触发区必须设置 MenuLabel。', '禁用控件仍然可见并暴露禁用语义。', '菜单内容、关闭与焦点恢复由应用负责。'] },
  },
};

const ownership: Omit<Section, 'ID'> = {
  Title: '同一份源码，两种预览',
  Text: '展示的代码来自构建时实际编译的 Go 示例文件，依赖 internal/componentdemo 中的 State，并非独立 main 程序。原生命令运行完整示例包。',
  Items: ['每个示例的重置仅创建新的预览实例，不修改生产组件。', '源码指纹标识 Go 构建输入，不是发布版本或跨平台一致性承诺。', 'Astro 开发服务在 Go 源码变更后重新编译并刷新页面。失败时保留上次成功预览；部署后的静态网站必须重新构建发布。'],
};

const genericChineseSections: Record<string, Omit<Section, 'ID'>> = {
  usage: {
    Title: '源码组件用法',
    Text: '此预览直接调用生产 Go 组件。稳定 ID、值与打开状态仍由应用持有；组件只负责绘制并声明交互语义。',
    Items: ['标题旁的源码链接列出了全部选项与视觉决策。', '请使用原生命令验证系统文本、焦点、无障碍与平台集成。', '浏览器预览验证共享布局和场景行为，不代表原生性能。'],
  },
  verification: {
    Title: '验证边界',
    Text: '示例覆盖当前 API 的代表性状态，但不表示每个桌面后端都已完成真机验证。',
    Items: [],
  },
};

function englishLabel(value: string): string {
  return value.split('-').map(word => word ? word[0].toUpperCase() + word.slice(1) : word).join(' ');
}

export function localizePage(page: Page, locale: Locale): Page {
  const translation = englishComponents[page.ID];
  const examples = page.Examples.map(example => {
    const translated = translation?.examples[example.ID] ?? [englishLabel(example.ID), `${page.Title} ${englishLabel(example.ID).toLowerCase()} example rendered by the production Go component.`];
    return { ...example, Title: locale === 'en' ? translated[0] : example.Title,
      Description: locale === 'en' ? translated[1] : example.Description };
  });
  return { ...page, Summary: locale === 'en' ? translation?.summary ?? `${page.Title} source-owned component states rendered by the production Go API.` : page.Summary, Examples: examples,
    Sections: page.Sections.map(section => {
      if (locale === 'en') return section;
      const translated = section.ID === 'source-ownership' ? ownership : chineseSections[page.ID]?.[section.ID] ?? genericChineseSections[section.ID];
      if (!translated) throw new Error(`Missing section translation: ${page.ID}/${section.ID}`);
      return { ID: section.ID, ...translated };
    }),
    Capabilities: locale === 'en' ? page.Capabilities : page.Capabilities.map(capability => {
      const translated: Record<string, [string, string]> = {
        macOS: ['原生预览', 'AppKit + Metal。请在本机验证系统字体、焦点与可访问性。'],
        Windows: ['实验性', '原生运行器通过交叉编译；本文档不是 Windows 真机验证报告。'],
        Linux: ['暂不可用', '当前 Linux 后端尚不能运行这些 Canvas 组件示例。'],
		Web: page.ID === 'input' ? ['交互式文本输入', '浏览器输入、选区与 IME 连接到原生预览使用的同一个应用持有 Go Editor。'] : ['Canvas2D 预览', '同一份 Go 示例编译为 WASM，不是 HTML/CSS 仿制，不能据此推断原生性能。'],
      };
      if (!translated[capability.Platform]) throw new Error(`Missing platform translation: ${capability.Platform}`);
      return { Platform: capability.Platform, Status: translated[capability.Platform][0], Detail: translated[capability.Platform][1] };
    }),
  };
}
