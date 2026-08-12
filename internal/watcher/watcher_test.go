package watcher

import "testing"

func TestFilenameFromSubject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		subject  string
		fallback string
		want     string
	}{
		{
			name:     "uses subject with original extension",
			subject:  "Monthly report",
			fallback: "invoice.pdf",
			want:     "Monthly report.pdf",
		},
		{
			name:     "falls back when subject is empty",
			subject:  "   ",
			fallback: "invoice.pdf",
			want:     "invoice.pdf",
		},
		{
			name:     "sanitizes path separators",
			subject:  "Q4/2026: summary",
			fallback: "report.docx",
			want:     "Q4-2026- summary.docx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := filenameFromSubject(tt.subject, tt.fallback); got != tt.want {
				t.Fatalf("filenameFromSubject() = %q, want %q", got, tt.want)
			}
		})
	}
}
