package app

import (
	"testing"

	"github.com/vivaui/viva/internal/diag"
	"github.com/vivaui/viva/internal/platform"
)

func captureDiagnostics(t *testing.T) *[]diag.Event {
	t.Helper()
	var events []diag.Event
	diag.SetHandler(func(event diag.Event) { events = append(events, event) })
	t.Cleanup(func() { diag.SetHandler(nil) })
	return &events
}

func TestAFaultReachesTheDiagnosticStream(t *testing.T) {
	events := captureDiagnostics(t)
	SetFaultHandler(nil)
	t.Cleanup(func() { SetFaultHandler(nil) })

	deliverFault(platform.Fault{Where: "window procedure", Message: "boom", Stack: []byte("goroutine 1")})

	if len(*events) != 1 {
		t.Fatalf("the stream saw %d events", len(*events))
	}
	event := (*events)[0]
	if event.Level != diag.Error || event.Subsystem != "app" || event.Code != "fault" || event.Message != "boom" {
		t.Errorf("event = %+v", event)
	}
	values := make(map[string]any, len(event.Attrs))
	for _, attr := range event.Attrs {
		values[attr.Key] = attr.Value.Any()
	}
	if values["where"] != "window procedure" || values["stack"] != "goroutine 1" {
		t.Errorf("attrs = %v", values)
	}
}

func TestAFaultAlsoReachesTheApplicationHandler(t *testing.T) {
	captureDiagnostics(t)
	var received Fault
	SetFaultHandler(func(fault Fault) { received = fault })
	t.Cleanup(func() { SetFaultHandler(nil) })

	deliverFault(platform.Fault{Where: "callback", Message: "boom", Stack: []byte("stack")})

	if received.Where != "callback" || received.Message != "boom" || string(received.Stack) != "stack" {
		t.Errorf("handler received %+v", received)
	}
}

func TestClearingTheApplicationHandlerKeepsTheStream(t *testing.T) {
	events := captureDiagnostics(t)
	SetFaultHandler(nil)

	deliverFault(platform.Fault{Where: "callback", Message: "boom"})

	if len(*events) != 1 {
		t.Errorf("the stream saw %d events after SetFaultHandler(nil)", len(*events))
	}
}

func resetFaultBridge(t *testing.T) {
	t.Helper()
	faultBridge.mu.Lock()
	faultBridge.installed = false
	platform.SetFaultHandler(nil)
	faultBridge.mu.Unlock()
	userFaultHandler.Store(nil)
	t.Cleanup(func() {
		faultBridge.mu.Lock()
		faultBridge.installed = false
		platform.SetFaultHandler(nil)
		faultBridge.mu.Unlock()
		userFaultHandler.Store(nil)
	})
}

func TestDiagnosticsActivatesTheFaultBridge(t *testing.T) {
	resetFaultBridge(t)
	diag.SetHandler(nil)
	diag.Capture(0)
	attendFaults()
	if faultBridge.installed {
		t.Fatal("the bridge was installed with nothing listening")
	}
	events := captureDiagnostics(t)
	if !faultBridge.installed {
		t.Fatal("a diagnostic handler did not install the bridge")
	}
	platform.ReportFault("test boundary", "boom")
	if len(*events) != 1 || (*events)[0].Code != "fault" {
		t.Errorf("events = %+v", *events)
	}
}

func TestClearingTheFaultHandlerRemovesTheBridge(t *testing.T) {
	resetFaultBridge(t)
	diag.SetHandler(nil)
	diag.Capture(0)
	SetFaultHandler(func(Fault) {})
	if !faultBridge.installed {
		t.Fatal("an application handler did not install the bridge")
	}
	SetFaultHandler(nil)
	if faultBridge.installed {
		t.Fatal("clearing the handler left the bridge installed with nothing listening")
	}
}
