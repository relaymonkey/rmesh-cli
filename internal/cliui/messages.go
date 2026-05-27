package cliui

// SavedCloudConfig reports a new personal cloud config row.
func (u *UI) SavedCloudConfig(label, path, id string) error {
	return u.Success("Saved cloud config · "+label,
		Field{Key: "path", Value: path},
		Field{Key: "id", Value: id},
	)
}

// UpdatedCloudConfig reports an in-place PATCH of an existing cloud row.
func (u *UI) UpdatedCloudConfig(label, path, id string) error {
	return u.Success("Updated cloud config · "+label,
		Field{Key: "path", Value: path},
		Field{Key: "id", Value: id},
	)
}

// PromotedCloudConfig reports personal → network template publication.
func (u *UI) PromotedCloudConfig(templateLabel, fromPath, toPath, id, visibility string) error {
	return u.Success("Promoted to network template · "+templateLabel,
		Field{Key: "from", Value: fromPath},
		Field{Key: "to", Value: toPath},
		Field{Key: "id", Value: id},
		Field{Key: "visibility", Value: visibility},
	)
}

// WroteFile reports a successful local file write-back.
func (u *UI) WroteFile(path string) error {
	return u.Success("Wrote file",
		Field{Key: "path", Value: path},
	)
}

// NoChanges reports an edit round-trip that did not alter the buffer.
func (u *UI) NoChanges(source string) error {
	return u.Status("No changes · source left untouched",
		Field{Key: "source", Value: source},
	)
}

// DryRun reports what would happen without mutating state.
func (u *UI) DryRun(headline string, fields ...Field) error {
	return u.Status("Dry run · "+headline, fields...)
}

// DeletedCloudConfig reports a removed personal or template row.
func (u *UI) DeletedCloudConfig(label, path, id string) error {
	return u.Success("Deleted cloud config · "+label,
		Field{Key: "path", Value: path},
		Field{Key: "id", Value: id},
	)
}
