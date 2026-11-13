package componentdemo

import (
	"math"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var graphComponent = Component{
	ID: "graph", Title: "Graph", Category: "Data Display",
	Summary:    "力导向双向链接图谱：调用方持有模拟与视口，悬停高亮邻居，拖动节点，滚轮缩放。",
	paint:      (*State).paintGraph,
	pointer:    (*State).pointerGraph,
	key:        (*State).keyGraph,
	scroll:     (*State).scrollGraph,
	initialize: (*State).initializeGraph,
	Examples: []Example{
		{
			ID: "vault", Title: "全局图谱",
			Description: "整个库的笔记与链接；悬停高亮邻居，其余压暗。",
			Height:      360,
			Snippet: `graph.layout = components.PaintGraph(canvas, theme, bounds, graph.sim,
    graph.nodes, graph.view, components.GraphOptions{
        Nodes: graphNodeList, SurfaceID: graphSurface})
if clicked, ok := graph.layout.Clicked(); ok {
    openNote(clicked) // the application owns what a node opens
}`,
		},
		{
			ID: "local", Title: "局部图谱",
			Description: "当前笔记两跳邻域的子图，同样的组件与状态。",
			Height:      360,
			Snippet: `// A local graph is the same component over a smaller simulation: the
// application extracts the two-hop neighborhood of the current note.
local := graphNeighborhood(sim, links, currentNote, 2)
components.PaintGraph(canvas, theme, bounds, local.sim, local.nodes,
    local.view, components.GraphOptions{Nodes: graphNodeList})`,
		},
	},
}

var (
	graphUI       = tree.Feature("graph")
	graphSurface  = graphUI.ID(tree.Name("surface"))
	graphNodeList = tree.Sub(graphUI, tree.Name("node"))
)

// graphDemoState is the caller-owned state of the graph examples: the
// simulation, the display data, the viewport, and the pointer bookkeeping a
// drag and a pan need between events.
type graphDemoState struct {
	sim     *components.GraphSim
	nodes   []components.GraphNode
	view    *components.GraphState
	layout  components.GraphLayout
	drag    int
	panning bool
	lastX   float32
	lastY   float32
	opened  string
}

// graphVaultLabels names the demo vault's notes in both preview languages.
// The structure is a hub-and-spoke vault: an index note, three groups each
// with its own head, chains inside a group, a few cross links, and two
// unresolved links to notes that do not exist.
var graphVaultGroups = [3]struct{ chinese, english string }{
	{"概念", "Concepts"}, {"工具", "Tools"}, {"人物", "People"},
}

func graphVaultLabels(english bool) []string {
	names := [...]string{"索引", "图谱", "链接", "卡片", "标签", "草稿", "归档", "日志",
		"编辑器", "终端", "同步", "备份", "剪贴板", "窗口", "主题", "快捷键",
		"高斯", "图灵", "香农", "维纳", "麦卡锡", "汉明", "未创建的引用", "待写的笔记"}
	if english {
		names = [...]string{"Index", "Graph", "Links", "Cards", "Tags", "Drafts", "Archive", "Journal",
			"Editor", "Terminal", "Sync", "Backup", "Clipboard", "Windows", "Themes", "Shortcuts",
			"Gauss", "Turing", "Shannon", "Wiener", "McCarthy", "Hamming", "Unresolved ref", "Note to write"}
	}
	return names[:]
}

func graphVaultLinks() []components.GraphLink {
	heads := [3]int{1, 8, 16}   // 概念/工具/人物 group heads
	limits := [3]int{7, 15, 21} // last index of each group
	links := make([]components.GraphLink, 0, 44)
	for group, head := range heads {
		links = append(links, components.GraphLink{Source: 0, Target: head})
		for index := head; index < limits[group]; index++ {
			links = append(links, components.GraphLink{Source: index, Target: index + 1})
		}
	}
	return append(links,
		components.GraphLink{Source: 2, Target: 9},
		components.GraphLink{Source: 5, Target: 22},
		components.GraphLink{Source: 11, Target: 17},
		components.GraphLink{Source: 12, Target: 23},
		components.GraphLink{Source: 19, Target: 4},
	)
}

// initializeGraph builds the demo vault, whole for the vault example and as
// the two-hop neighborhood of the 索引 note's 概念 head for the local one.
func (state *State) initializeGraph() {
	english := state.english
	labels := graphVaultLabels(english)
	links := graphVaultLinks()
	demo := &graphDemoState{
		nodes: make([]components.GraphNode, len(labels)),
		view:  &components.GraphState{},
		drag:  -1,
	}
	for index := range demo.nodes {
		demo.nodes[index] = components.GraphNode{Label: labels[index], Group: graphVaultGroup(index), Resolved: index < 22}
	}
	if state.exampleID == "local" {
		keep := graphNeighborhood(len(labels), links, 1, 2)
		remap := make([]int, len(labels))
		for index := range remap {
			remap[index] = -1
		}
		nodes := make([]components.GraphNode, 0, len(keep))
		for index := range demo.nodes {
			if !keep[index] {
				continue
			}
			remap[index] = len(nodes)
			nodes = append(nodes, demo.nodes[index])
		}
		local := make([]components.GraphLink, 0, len(links))
		for _, link := range links {
			if keep[link.Source] && keep[link.Target] {
				local = append(local, components.GraphLink{Source: remap[link.Source], Target: remap[link.Target]})
			}
		}
		demo.nodes, links = nodes, local
	}
	demo.sim = components.NewGraphSim(len(demo.nodes), links)
	state.graph = demo
}

// graphNeighborhood reports which of count nodes lie within depth hops of
// from, by a breadth-first walk over links.
func graphNeighborhood(count int, links []components.GraphLink, from, depth int) []bool {
	keep := make([]bool, count)
	if from < 0 || from >= count {
		return keep
	}
	keep[from] = true
	frontier := []int{from}
	for step := 0; step < depth && len(frontier) > 0; step++ {
		next := frontier[:0:0]
		for _, node := range frontier {
			for _, link := range links {
				other, isLink := link.Source, link.Target == node
				if link.Source == node {
					other, isLink = link.Target, true
				}
				if isLink && other >= 0 && other < count && !keep[other] {
					keep[other] = true
					next = append(next, other)
				}
			}
		}
		frontier = next
	}
	return keep
}

// graphVaultGroup is the color group a note of the demo vault belongs to.
func graphVaultGroup(index int) int {
	switch {
	case index == 0:
		return 0
	case index < 8:
		return 0
	case index < 16:
		return 1
	default:
		return 2
	}
}

func (state *State) paintGraph(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	graph := state.graph
	var card ui.Box
	var opened ui.Box
	state.drawPage(canvas, theme, size, 680, state.Text("Graph · 双向链接图谱", "Graph · Backlink graph"), func(c ui.C) {
		card = ui.Slot(c).FillWidth().Height(320)
		caption(c, state.Text("悬停高亮邻居；拖动节点或背景；滚轮缩放；方向键在邻居间移动。",
			"Hover highlights neighbors; drag a node or the background; scroll zooms; arrows step neighbors."))
		opened = ui.Slot(c).FillWidth().Height(18)
	})
	bounds := card.Rect()
	components.PaintCard(canvas, theme, bounds, components.CardOptions{})
	inner := gfx.Rect{X: bounds.X + 8, Y: bounds.Y + 8, Width: bounds.Width - 16, Height: bounds.Height - 16}
	graph.layout = components.PaintGraph(canvas, theme, inner, graph.sim, graph.nodes, graph.view, components.GraphOptions{
		Nodes:     graphNodeList,
		SurfaceID: graphSurface,
	})
	if clicked, ok := graph.layout.Clicked(); ok {
		graph.opened = graph.nodes[clicked].Label
	}
	if graph.opened != "" {
		canvas.Label(state.Text("已打开：", "Opened: ")+graph.opened, opened.Rect(),
			gfx.TextStyle{Size: theme.Metrics.FontSizeS, Color: theme.MutedForeground})
	}
}

// pointerGraph drags a node (the simulation follows while it is held), pans
// the viewport from the background, and keeps the node under the keyboard
// focused while it is dragged.
func (state *State) pointerGraph(event gfx.PointerEvent) gfx.EventResult {
	graph := state.graph
	if graph == nil {
		return gfx.EventResult{}
	}
	switch event.Kind {
	case gfx.PointerDown:
		if event.Button != gfx.PointerButtonPrimary {
			return gfx.EventResult{}
		}
		if node, ok := event.Target.In(graphNodeList); ok {
			graph.drag = node
			x, y := graph.layout.FromScreen(event.X, event.Y)
			graph.sim.SetFixed(node, x, y)
			// d3 keeps the layout warm for the whole drag, so the neighbors
			// give way while the node travels instead of after the release.
			graph.sim.Reheat(0.3)
			graph.view.Select(node)
			return gfx.EventResult{Focus: graphNodeList.At(node), Handled: true, Repaint: true}
		}
		if event.Target == graphSurface {
			graph.panning = true
			graph.lastX, graph.lastY = event.X, event.Y
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		return gfx.EventResult{}
	case gfx.PointerMove:
		if !event.Captured {
			return gfx.EventResult{}
		}
		if graph.drag >= 0 {
			x, y := graph.layout.FromScreen(event.X, event.Y)
			graph.sim.SetFixed(graph.drag, x, y)
			graph.sim.Reheat(0.3)
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if graph.panning {
			graph.view.Pan.X += event.X - graph.lastX
			graph.view.Pan.Y += event.Y - graph.lastY
			graph.lastX, graph.lastY = event.X, event.Y
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		return gfx.EventResult{}
	case gfx.PointerUp, gfx.PointerExit:
		if graph.drag >= 0 {
			graph.sim.SetFree(graph.drag)
			graph.sim.Reheat(0.3)
			graph.drag = -1
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if graph.panning {
			graph.panning = false
			return gfx.EventResult{Handled: true}
		}
	}
	return gfx.EventResult{}
}

// scrollGraph zooms about the pointer, keeping the graph point under it in
// place, from whatever the current zoom is — the fit zoom included.
func (state *State) scrollGraph(event gfx.ScrollEvent) gfx.EventResult {
	graph := state.graph
	if graph == nil {
		return gfx.EventResult{}
	}
	bounds := graph.layout.Bounds
	if event.X < bounds.X || event.X > bounds.X+bounds.Width || event.Y < bounds.Y || event.Y > bounds.Y+bounds.Height {
		return gfx.EventResult{}
	}
	current := graph.view.Zoom
	if current <= 0 {
		current = graph.layout.Scale
	}
	next := min(max(current*float32(math.Exp(float64(-event.DeltaY*0.0022))), 0.15), 4)
	centerX, centerY := bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2
	underX := (event.X - centerX - graph.view.Pan.X) / current
	underY := (event.Y - centerY - graph.view.Pan.Y) / current
	graph.view.Zoom = next
	graph.view.Pan.X = event.X - centerX - underX*next
	graph.view.Pan.Y = event.Y - centerY - underY*next
	return gfx.EventResult{Handled: true, Repaint: true}
}

// keyGraph moves the selection through the focused node's neighbors; the
// runtime delivers Enter and Space as the node's click.
func (state *State) keyGraph(event gfx.KeyEvent) gfx.EventResult {
	graph := state.graph
	if graph == nil || event.Release {
		return gfx.EventResult{}
	}
	node, ok := event.Target.In(graphNodeList)
	if !ok {
		return gfx.EventResult{}
	}
	next, moved := components.GraphStepForKey(event.Key, node, graph.sim.Neighbors(node))
	if !moved {
		return gfx.EventResult{}
	}
	graph.view.Select(next)
	return gfx.EventResult{Focus: graphNodeList.At(next), FocusVisible: true, Handled: true, Repaint: true}
}
