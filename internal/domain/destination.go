package domain
import "time"

type Destination struct {
  ID          int64     `db:"id" json:"id"`
  Name        string    `db:"name" json:"name"`
  City        string    `db:"city" json:"city"`
  Category    string    `db:"category" json:"category"` // park, beach, museum...
  Description string    `db:"description" json:"description"`
  Latitude    float64   `db:"lat" json:"lat"`
  Longitude   float64   `db:"lng" json:"lng"`
  RatingAvg   float64   `db:"rating_avg" json:"rating_avg"`
  CreatedAt   time.Time `db:"created_at" json:"created_at"`
}