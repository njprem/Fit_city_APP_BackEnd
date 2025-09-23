type Review struct {
  ID            int64     `db:"id" json:"id"`
  DestinationID int64     `db:"destination_id" json:"destination_id"`
  UserID        int64     `db:"user_id" json:"user_id"`
  Rating        int       `db:"rating" json:"rating"` // 1..5
  Comment       string    `db:"comment" json:"comment"`
  CreatedAt     time.Time `db:"created_at" json:"created_at"`
}
