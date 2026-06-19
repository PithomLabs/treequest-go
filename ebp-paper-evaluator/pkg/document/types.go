package document

type SourceMetadata struct {
	Kind         string `json:"kind"`
	InputPath    string `json:"input_path"`
	ResolvedPath string `json:"resolved_path,omitempty"`
	MediaType    string `json:"media_type"`
	OriginalHash string `json:"original_hash"`
	SizeBytes    int64  `json:"size_bytes"`
}
type SourceRef struct {
	Page  int `json:"page,omitempty"`
	Start int `json:"start"`
	End   int `json:"end"`
}
type Section struct {
	ID         string      `json:"id"`
	Title      string      `json:"title"`
	Ordinal    int         `json:"ordinal"`
	Text       string      `json:"text"`
	Pages      []int       `json:"pages,omitempty"`
	SourceRefs []SourceRef `json:"source_refs,omitempty"`
}
type Chunk struct {
	ID         string `json:"id"`
	SectionID  string `json:"section_id"`
	Text       string `json:"text"`
	Start      int    `json:"start"`
	End        int    `json:"end"`
	Pages      []int  `json:"pages,omitempty"`
	TokenCount int    `json:"token_count"`
	SourceHash string `json:"source_hash"`
}
type DocumentBundle struct {
	ID       string         `json:"id"`
	Title    string         `json:"title"`
	Abstract string         `json:"abstract"`
	Sections []Section      `json:"sections"`
	Chunks   []Chunk        `json:"chunks"`
	Source   SourceMetadata `json:"source"`
	Hash     string         `json:"hash"`
	Summary  string         `json:"summary"`
}
type EvidenceSpan struct {
	ID         string `json:"id"`
	Section    string `json:"section"`
	Page       int    `json:"page,omitempty"`
	Start      int    `json:"start"`
	End        int    `json:"end"`
	Quote      string `json:"quote"`
	SourceHash string `json:"source_hash"`
}

type PaperInput struct {
	Path                string
	InputRoot           string
	MaxBytes            int64
	IncludeResolvedPath bool
}
