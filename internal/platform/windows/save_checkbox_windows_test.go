package windows

import "testing"

func TestSaveDialogCheckboxControlStartsWithRequestedState(t *testing.T) {
	for _, initial := range []bool{false, true} {
		err := withSTA(func() error {
			dialog, err := createFileDialog(&saveDialogClass, &saveDialogInterface)
			if err != nil {
				return err
			}
			defer release(&dialog)
			custom, err := addSaveDialogCheckbox(dialog, "Save Folder as Workspace", initial)
			if err != nil {
				return err
			}
			defer release(&custom)
			checked, err := saveDialogCheckboxState(custom)
			if err != nil {
				return err
			}
			if checked != initial {
				t.Fatalf("save checkbox = %v, want %v", checked, initial)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
