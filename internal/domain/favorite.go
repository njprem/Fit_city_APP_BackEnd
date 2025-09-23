type Favorite struct {
  UserID        int64 `db:"user_id" json:"user_id"`
  DestinationID int64 `db:"destination_id" json:"destination_id"`
}