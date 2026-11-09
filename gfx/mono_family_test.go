package gfx

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func resetMonoFaces() { monoStacks.Store(nil) }

func TestMonoFamilyIsStableAndBounded(t *testing.T) {
	resetMonoFaces()
	first := MonoFamily("JetBrains Mono")
	if first != 2 || MonoFamily(" JetBrains Mono ") != first {
		t.Fatalf("first face = %d", first)
	}
	if second := MonoFamily("Menlo"); second != 3 || MonoFamilyName(second) != "Menlo" || MonoFamilyName(first) != "JetBrains Mono" {
		t.Fatalf("second face = %d", second)
	}
	for _, bad := range []string{"", "   ", "a\x00b", "line\nbreak", "a,b", strings.Repeat("x", 97)} {
		if MonoFamily(bad) != MonoSystem {
			t.Errorf("%q was registered", bad)
		}
	}
	for index := 0; index < MaxMonoFaces+5; index++ {
		MonoFamily(fmt.Sprintf("Family %d", index))
	}
	if MonoFaceCount() != MaxMonoFaces || MonoFamily("One too many") != MonoSystem {
		t.Fatalf("registry holds %d", MonoFaceCount())
	}
	if MonoFamilyName(MonoSystem) != "" || MonoFamilyName(MonoClassic) != "" || MonoFamilyName(65535) != "" {
		t.Fatal("a built-in or unknown face has a name")
	}
}

func TestMonoFamilyConcurrentRegistrationAgreesOnFaces(t *testing.T) {
	resetMonoFaces()
	var wait sync.WaitGroup
	faces := make([]MonoFace, 16)
	for index := range faces {
		wait.Add(1)
		go func() {
			defer wait.Done()
			faces[index] = MonoStack(MonoFamily("Shared Family"), "Fallback")
		}()
	}
	wait.Wait()
	for _, face := range faces {
		if face != faces[0] || MonoFamilyName(face) != "Shared Family" || len(MonoFallbacks(face)) != 1 {
			t.Fatalf("faces = %v", faces)
		}
	}
	if MonoFaceCount() != 2 { // the family alone, and the family with its fallback
		t.Fatalf("count = %d", MonoFaceCount())
	}
}

func TestMonoStackKeepsTheBaseAndOrdersTheFallbacks(t *testing.T) {
	resetMonoFaces()
	base := MonoFamily("Maple Mono NF")
	stack := MonoStack(base, "LXGW WenKai Mono Lite", " Symbols Nerd Font ")
	if stack == base || MonoFamilyName(stack) != "Maple Mono NF" {
		t.Fatalf("stack %d over %d is in %q", stack, base, MonoFamilyName(stack))
	}
	if got := MonoFallbacks(stack); len(got) != 2 || got[0] != "LXGW WenKai Mono Lite" || got[1] != "Symbols Nerd Font" {
		t.Fatalf("fallbacks = %q", got)
	}
	if again := MonoStack(base, "LXGW WenKai Mono Lite", "Symbols Nerd Font"); again != stack {
		t.Fatalf("the same stack got %d then %d", stack, again)
	}
	if swapped := MonoStack(base, "Symbols Nerd Font", "LXGW WenKai Mono Lite"); swapped == stack {
		t.Fatal("the order of the fallbacks does not tell stacks apart")
	}
	if MonoFallbacks(base) != nil {
		t.Fatal("a plain family has fallbacks")
	}
}

func TestMonoStackIsBuiltOnABuiltInFaceOrAStackAndDropsWhatCannotBeUsed(t *testing.T) {
	resetMonoFaces()
	system := MonoStack(MonoSystem, "Noto Sans CJK SC")
	classic := MonoStack(MonoClassic, "Noto Sans CJK SC")
	if system == classic || MonoFamilyName(system) != "" || MonoIsClassic(system) || !MonoIsClassic(classic) {
		t.Fatalf("system stack %d classic %d", system, classic)
	}
	if MonoStack(MonoSystem) != MonoSystem || MonoStack(MonoClassic) != MonoClassic || MonoStack(MonoSystem, "", "a,b", "x\ny") != MonoSystem {
		t.Fatal("a stack without usable fallbacks is not its base")
	}
	base := MonoFamily("Menlo")
	if MonoStack(base) != base || MonoStack(base, "Menlo", " Menlo ") != base {
		t.Fatal("the base's own family is a fallback of itself")
	}
	deep := MonoStack(MonoStack(base, "A"), "B")
	if got := MonoFallbacks(deep); len(got) != 1 || got[0] != "B" || MonoFamilyName(deep) != "Menlo" {
		t.Fatalf("restacking kept %q over %q", got, MonoFamilyName(deep))
	}
	var many []string
	for index := 0; index < MaxMonoFallbacks+4; index++ {
		many = append(many, fmt.Sprintf("Family %d", index))
	}
	if got := MonoFallbacks(MonoStack(base, append(many, many[0], "Family 1")...)); len(got) != MaxMonoFallbacks || got[MaxMonoFallbacks-1] != "Family 7" {
		t.Fatalf("kept %q", got)
	}
	if MonoStack(200, "A") == 200 || MonoFamilyName(MonoStack(200, "A")) != "" {
		t.Fatal("an unregistered base was kept")
	}
}

func TestMonoStackReadsDoNotAllocate(t *testing.T) {
	resetMonoFaces()
	face := MonoStack(MonoFamily("Menlo"), "Fallback One", "Fallback Two")
	allocs := testing.AllocsPerRun(100, func() {
		if MonoStack(MonoFamily("Menlo"), "Fallback One", "Fallback Two") != face || len(MonoFallbacks(face)) != 2 || MonoFamilyName(face) == "" || !validMonoFace(face) {
			t.Fatal("lookup changed")
		}
	})
	if allocs != 0 {
		t.Fatalf("lookup allocates %v times", allocs)
	}
}

func BenchmarkMonoStackLookup(b *testing.B) {
	resetMonoFaces()
	base := MonoFamily("Maple Mono NF")
	b.ReportAllocs()
	for b.Loop() {
		MonoStack(base, "LXGW WenKai Mono Lite", "Symbols Nerd Font")
	}
}

func BenchmarkMonoFallbacksRead(b *testing.B) {
	resetMonoFaces()
	face := MonoStack(MonoFamily("Maple Mono NF"), "LXGW WenKai Mono Lite")
	b.ReportAllocs()
	for b.Loop() {
		_ = MonoFallbacks(face)
		_ = MonoFamilyName(face)
	}
}

func TestACanvasMeasuresARegisteredFaceByItsBaseAndTreatsAnUnknownFaceAsTheSystemOne(t *testing.T) {
	resetMonoFaces()
	var canvas Canvas
	asked := map[MonoFace]int{}
	canvas.SetCellMeasurer(func(size float32, face MonoFace) (float32, float32) {
		asked[face]++
		return 7, 15
	})
	face := MonoStack(MonoFamily("Test Mono"), "Test CJK")
	canvas.MonoCellSize(13, face)
	canvas.MonoCellSize(13, 9) // not registered
	if asked[face] != 1 || asked[MonoSystem] != 1 {
		t.Fatalf("measurer asked %v", asked)
	}
}
