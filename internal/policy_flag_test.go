package internal

import "testing"

func TestFlag_SetRoot(t *testing.T) {
	f := Flag{}
	f.SetRoot(true)
	if !f.IsRoot() {
		t.Error("Expected root flag to be true, got false")
	}
	f.SetRoot(false)
	if f.IsRoot() {
		t.Error("Expected root flag to be false, got true")
	}
}

func TestFlag_SetProbation(t *testing.T) {
	f := Flag{}
	f.SetProbation(true)
	if !f.IsProbation() {
		t.Error("Expected probation flag to be true, got false")
	}
	f.SetProbation(false)
	if f.IsProbation() {
		t.Error("Expected probation flag to be false, got true")
	}
}

func TestFlag_SetProtected(t *testing.T) {
	f := Flag{}
	f.SetProtected(true)
	if !f.IsProtected() {
		t.Error("Expected protected flag to be true, got false")
	}
	f.SetProtected(false)
	if f.IsProtected() {
		t.Error("Expected protected flag to be false, got true")
	}
}

func TestFlag_SetRemoved(t *testing.T) {
	f := Flag{}
	f.SetRemoved(true)
	if !f.IsRemoved() {
		t.Error("Expected removed flag to be true, got false")
	}
	f.SetRemoved(false)
	if f.IsRemoved() {
		t.Error("Expected removed flag to be false, got true")
	}
}

func TestFlag_SetFromNVM(t *testing.T) {
	f := Flag{}
	f.SetFromNVM(true)
	if !f.IsFromNVM() {
		t.Error("Expected from NVM flag to be true, got false")
	}
	f.SetFromNVM(false)
	if f.IsFromNVM() {
		t.Error("Expected from NVM flag to be false, got true")
	}
}

func TestFlag_CombinedFlags(t *testing.T) {
	f := Flag{}
	f.SetRoot(true)
	f.SetProbation(true)
	f.SetProtected(false)
	f.SetRemoved(true)
	f.SetFromNVM(true)

	if !f.IsRoot() {
		t.Error("Expected root flag to be true, got false")
	}
	if !f.IsProbation() {
		t.Error("Expected probation flag to be true, got false")
	}
	if f.IsProtected() {
		t.Error("Expected protected flag to be false, got true")
	}
	if !f.IsRemoved() {
		t.Error("Expected removed flag to be true, got false")
	}
	if !f.IsFromNVM() {
		t.Error("Expected from NVM flag to be true, got false")
	}

	// reset
	f.flags = 0

	if f.IsRoot() {
		t.Error("Expected root flag to be false, got true")
	}
	if f.IsProbation() {
		t.Error("Expected probation flag to be false, got true")
	}
	if f.IsProtected() {
		t.Error("Expected protected flag to be false, got true")
	}
	if f.IsRemoved() {
		t.Error("Expected removed flag to be false, got true")
	}
	if f.IsFromNVM() {
		t.Error("Expected from NVM flag to be false, got true")
	}
}
