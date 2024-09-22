package internal

// Flag struct uses 8 bits, with each bit representing a boolean value.
// Currently, 5 bits are used.
// All bits are read/write in policy only(with policy mutex), so safe to put them together.
// Bit 1: Indicates if this entry is a root of linked list.
// Bit 2: Indicates if this entry is on probation.
// Bit 3: Indicates if this entry is protected.
// Bit 4: Indicates if this entry is removed.
// Bit 5: Indicates if this entry is from NVM.
type Flag struct {
	flags int8
}

func (f *Flag) SetRoot(isRoot bool) {
	if isRoot {
		f.flags |= (1 << 0) // Set bit 1 (root)
	} else {
		f.flags &^= (1 << 0) // Clear bit 1 (root)
	}
}

func (f *Flag) SetProbation(isProbation bool) {
	if isProbation {
		f.flags |= (1 << 1) // Set bit 2 (probation)
	} else {
		f.flags &^= (1 << 1) // Clear bit 2 (probation)
	}
}

func (f *Flag) SetProtected(isProtected bool) {
	if isProtected {
		f.flags |= (1 << 2) // Set bit 3 (protected)
	} else {
		f.flags &^= (1 << 2) // Clear bit 3 (protected)
	}
}

func (f *Flag) SetRemoved(isRemoved bool) {
	if isRemoved {
		f.flags |= (1 << 3) // Set bit 4 (removed)
	} else {
		f.flags &^= (1 << 3) // Clear bit 4 (removed)
	}
}

func (f *Flag) SetFromNVM(isFromNVM bool) {
	if isFromNVM {
		f.flags |= (1 << 4) // Set bit 5 (from NVM)
	} else {
		f.flags &^= (1 << 4) // Clear bit 5 (from NVM)
	}
}

func (f *Flag) IsRoot() bool {
	return (f.flags & (1 << 0)) != 0
}

func (f *Flag) IsProbation() bool {
	return (f.flags & (1 << 1)) != 0
}

func (f *Flag) IsProtected() bool {
	return (f.flags & (1 << 2)) != 0
}

func (f *Flag) IsRemoved() bool {
	return (f.flags & (1 << 3)) != 0
}

func (f *Flag) IsFromNVM() bool {
	return (f.flags & (1 << 4)) != 0
}
