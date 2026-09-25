package models

import "time"

type Genre struct {
	ID    int64
	Name  string
	Count int
}

var DefaultGenres = []string{
	"Histoire",
	"Capital",
	"Antifascisme",
	"Anarchisme",
	"Marxisme",
	"Jeunesse",
	"Magazine",
	"Parentalité",
	"Dev",
	"Sport",
	"Mangas",
	"Classiques",
	"Romans",
	"Science-Fiction",
	"Polar",
	"Fantasy",
	"Colonisation",
	"Internationale",
}

type ReadingStatus string

const (
	StatusInProgress ReadingStatus = "in_progress"
	StatusToRead     ReadingStatus = "to_read"
	StatusToBuy      ReadingStatus = "to_buy"
	StatusFinished   ReadingStatus = "finished"
)

func IsValidReadingStatus(status ReadingStatus) bool {
	switch status {
	case StatusInProgress, StatusToRead, StatusToBuy, StatusFinished:
		return true
	default:
		return false
	}
}

type Book struct {
	ID                   int64
	OpenLibraryEditionID string
	GenreID              *int64
	GenreName            string
	ReadingStatus        ReadingStatus
	Title                string
	Author               string
	CoverURL             string
	IsNextPal            bool
	CreatedAt            time.Time
}
