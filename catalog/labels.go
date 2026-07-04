package catalog

import "database/sql"

// maxLabelDepth bounds the label_relationship parent walk (cycle-guarded); deep
// sublabel chains beyond this fall back to the deepest resolved ancestor.
const maxLabelDepth = 8

// labelRoot walks label_relationship (parent_label_id → sublabel_id) upward from a
// label to its family root: the topmost ancestor with no parent, within maxLabelDepth.
func (m *MirrorDB) labelRoot(labelID int64) (int64, error) {
	cur := labelID
	seen := map[int64]bool{cur: true}
	for i := 0; i < maxLabelDepth; i++ {
		var parent int64
		err := m.DB.QueryRow(
			`SELECT parent_label_id FROM dc.label_relationship WHERE sublabel_id = ? ORDER BY parent_label_id LIMIT 1`,
			cur).Scan(&parent)
		if err != nil {
			// sql.ErrNoRows → no parent → cur is the root. Any other error propagates.
			if isNoRows(err) {
				return cur, nil
			}
			return 0, err
		}
		if seen[parent] {
			return cur, nil // cycle guard
		}
		seen[parent] = true
		cur = parent
	}
	return cur, nil
}

// labelIDByName resolves a Discogs label name to its id (casefold exact match).
// ok=false when no dc.label row matches.
func (m *MirrorDB) labelIDByName(name string) (int64, bool, error) {
	var id int64
	err := m.DB.QueryRow(`SELECT id FROM dc.label WHERE name = ? COLLATE NOCASE LIMIT 1`, name).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

// sameLabelFamily reports whether two labels share a family root.
func (m *MirrorDB) sameLabelFamily(a, b int64) (bool, error) {
	if a == b {
		return true, nil
	}
	ra, err := m.labelRoot(a)
	if err != nil {
		return false, err
	}
	rb, err := m.labelRoot(b)
	if err != nil {
		return false, err
	}
	return ra == rb, nil
}
