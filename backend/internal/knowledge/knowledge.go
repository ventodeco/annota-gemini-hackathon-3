package knowledge

// Entry represents a Japanese vocabulary term from the CSV knowledge base.
type Entry struct {
	Kosakata        string   // Japanese term (kanji) - primary lookup key
	Kana            string   // Hiragana reading
	Arti            string   // Meaning (EN/ID)
	CaraBaca        string   // Romaji pronunciation
	Deskripsi       string   // Detailed description in Indonesian
	BidangPekerjaan []string // Work fields
	Industri        []string // Industries
	Konteks         string   // Additional context
}

// Service provides vocabulary lookup functionality.
type Service interface {
	// Lookup finds entries matching the text (exact or substring).
	Lookup(text string) []Entry
}
