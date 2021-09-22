package routing

// TableAndVersionHistory holds both a routing table
// and a history of its versions
type TableAndVersionHistory struct {
	*Table
	*TableVersionHistory
}

// NewEmptyTableAndVersionHistory creates and returns
// a new instance of a TableAndVersionHistory with an
// empty routing table and an empty history in it.
func NewEmptyTableAndVersionHistory() TableAndVersionHistory {
	return TableAndVersionHistory{
		Table:               NewTable(),
		TableVersionHistory: NewTableVersionHistory(),
	}
}

// ReplaceTable replaces target's routing table data
// with newTable. It then adds newVsn to the given
// TableVersionWriter.
//
// This function is concurrency safe for target, but not
// necessarily for newTable.
// The caller must ensure that any concurrent accesses to
// newTable are protected
func ReplaceTable(
	target TableAndVersionHistory,
	newTable *Table,
	newVsn string,
) error {
	target.l.Lock()
	defer target.l.Unlock()
	target.m = newTable.m
	return target.AddVersion(newVsn)
}