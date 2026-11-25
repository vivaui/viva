// Chinese descriptions for the API tables on component pages. The Go doc
// comments remain the English source of truth and fill any key missing here;
// this dictionary only translates the pages that have tables today. Keys are
// "package.Symbol" for a type or function, and "package.Symbol.Field" or
// "package.Symbol.Method" for one row.
export const apiZh: Record<string, string> = {
  'ui.Toaster': 'Toaster 保存一个窗口的通知栈，放在模型里，用法与 shadcn 的 Sonner 相同：Show 放上一条通知，页面里声明一次的 ui.Toasts 负责绘制。',
  'ui.Toaster.Position': '通知栈的位置；零值是右下角。',
  'ui.Toaster.Expanded': '让通知栈保持展开，效果等同于指针停在栈上或焦点进入栈内：由调用方持有的标志，用于无法悬停的静态页面。零值是折叠。',
  'ui.Toaster.Timeout': '没有单独设置时长的通知使用这个时长；零值为 5 秒，与 Base UI 一致。',
  'ui.Toaster.Limit': '同时可见的通知条数上限；零值为 3，与 Base UI 一致。',
  'ui.Toaster.Acted': 'Acted 报告本次绘制中按下的是哪条通知的操作按钮；在 ui.Toasts 之后读取。',
  'ui.Toaster.Close': 'Close 在通知到期前把它收起。',
  'ui.Toaster.Done': 'Done 把一条 ToastLoading 通知替换成它最终的结果：立即更新标题、描述和类型，并从现在开始重新计时。',
  'ui.Toaster.ID': 'ID 返回第 n 条通知的标识；Toaster 首次绘制前为零。',
  'ui.Toaster.Len': 'Len 是当前通知条数。',
  'ui.Toaster.Show': 'Show 放上一条通知：一个标题，以及标题下方可选的描述（传空字符串表示没有）。相同标题和描述已在栈上时不重复显示，只重新计时。',
  'ui.Toaster.ShowAction': 'ShowAction 是带操作按钮的 Show，按钮文字由 action 指定；按下后 Acted 会报告，并把通知收起，例如 Undo、Retry、Open。操作按钮不是唯一入口：通知会自己到期，所以它提供的内容在窗口里还应有别处可访问。',
  'ui.Toaster.ShowConfig': 'ShowConfig 是 Show 加上这条通知单独的设置：操作按钮、自己的时长，或隐藏关闭按钮。隐藏关闭按钮的通知仍会按时到期、被操作按钮收起，或由应用调用 Close 收起。',
  'ui.Toaster.Swipe': 'Swipe 让第 id 条通知在本次及之后的绘制中水平移动 x 像素：这是调用方提供的拖拽快照，ui.Toasts 会原样绘制。正数向右，零复位。它不读取指针、不解释手势、不调度帧；拖拽和阈值由调用方负责。通知不在栈上时什么也不做。',
  'ui.Toaster.SwipeX': 'SwipeX 返回 Swipe 会让第 id 条通知在下次绘制中移动的距离；通知不在栈上时为零。',

  'ui.ToastConfig': 'ToastConfig 是一次 show 调用为单条通知设置的额外内容：操作按钮、自己的时长，或隐藏关闭按钮。',
  'ui.ToastConfig.Action': '操作按钮的文字；为空表示没有按钮。',
  'ui.ToastConfig.Timeout': '覆盖 Toaster 的 Timeout，只作用于这条通知；零或负数表示沿用 Toaster 的时长。',
  'ui.ToastConfig.HideClose': '隐藏关闭按钮。通知仍会按时到期、被操作按钮收起，或由应用调用 Close 收起。',

  'ui.Toasts': 'Toasts 在 t.Position 指定的位置绘制 t 的通知栈，盖在其他内容之上，并在到期时收起。它只在需要下一帧时请求一帧，没有通知时不调度任何帧。在页面里声明一次，放在最后。',

  'components.PaintToast': 'PaintToast 在 options 指定的位置绘制一条通知快照，默认右下角。宽度跟随文字，最宽 384px；高度跟随描述，描述在该宽度内换行。它不调度工作，也不保留生命周期状态；唯一的例外是 loading 通知每 100ms 请求一帧来转动 spinner，直到它不再被绘制。',

  'components.ToastOptions': 'ToastOptions 保存稳定的操作和浮层标识。',
  'components.ToastOptions.Layer': '通知绘制的画布图层。',
  'components.ToastOptions.Scope': '把通知的标识和焦点归入同一作用域。',
  'components.ToastOptions.DismissID': '关闭按钮和 Escape 目标的标识。为零时不绘制关闭按钮，Escape 交给页面处理。',
  'components.ToastOptions.ActionID': '操作按钮的标识；为零时不绘制操作按钮。',
  'components.ToastOptions.ActionLabel': '操作按钮的文字；为空时不绘制操作按钮。',
  'components.ToastOptions.Disabled': '让通知变暗，并禁用操作和关闭按钮。',
  'components.ToastOptions.Position': '通知的位置；零值是右下角。',

  'components.ToastSnapshot': 'ToastSnapshot 是一帧里绘制的完整调用方状态。计时、过期、滑动解释和栈成员关系都在组件之外。',
  'components.ToastSnapshot.Title': '通知的第一行文字。',
  'components.ToastSnapshot.Description': '标题下方较小的一行文字；为空时不绘制。',
  'components.ToastSnapshot.Type': '选择图标、图标颜色和 spinner。',
  'components.ToastSnapshot.Index': '通知在栈中位于最前面之后的第几层。',
  'components.ToastSnapshot.Expanded': '展开通知栈时用间隙而不是堆叠露出。',
  'components.ToastSnapshot.Limited': '标记超出可见上限的通知；它不会被绘制。',
  'components.ToastSnapshot.OffsetY': '调用方提供的纵向快照偏移。',
  'components.ToastSnapshot.SwipeX': '调用方提供的横向拖拽快照。',
  'components.ToastSnapshot.SwipeY': '调用方提供的纵向拖拽快照。',

  'components.PaintSonner': 'PaintSonner 使用官方的主题化 Toast 配方绘制一个完整的调用方通知栈。它不保留条目、不运行计时器、不调度帧，也不解释滑动手势。',
  'components.SonnerOptions': 'SonnerOptions 配置调用方通知栈的一次无状态绘制。通知按最新在前排序。Layer 是栈中最旧一条的图层，更新的一条依次加一，折叠卡片才能正确合成。',
  'components.SonnerOptions.Layer': '栈中最旧一条通知的图层；更新的通知依次加一。',
  'components.SonnerOptions.Expanded': '展开通知栈时用间隙而不是堆叠露出。',
  'components.SonnerOptions.Limit': '只显示最新的几条通知；零值为 Base UI 的 3，更旧的不会被绘制。',
  'components.SonnerOptions.Position': '通知栈增长的位置；零值是右下角。',

  'components.SonnerToast': 'SonnerToast 是调用方通知栈里的一条通知：它的快照，以及关闭和操作按钮的标识。',
  'components.SonnerToast.Snapshot': '通知的文字、类型和位置。',
  'components.SonnerToast.Scope': '把通知的标识和焦点归入同一作用域。',
  'components.SonnerToast.DismissID': '关闭按钮的标识；为零时不绘制关闭按钮。',
  'components.SonnerToast.ActionID': '操作按钮的标识；为零时不绘制操作按钮。',
  'components.SonnerToast.ActionLabel': '操作按钮的文字；为空时不绘制操作按钮。',
  'components.SonnerToast.Disabled': '让通知变暗，并禁用操作和关闭按钮。',
};
