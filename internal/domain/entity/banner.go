package entity

import (
	"encoding/json"
	"time"
)

type Banner struct {
	BannerID  int64
	TagIDs    []int64       `json:"tag_ids"`
	FeatureID int64         `json:"feature_id"`
	Content   BannerContent `json:"content"`
	IsActive  bool          `json:"is_active"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

type BannerContent struct {
	Title string `json:"title" redis:"title"`
	Text  string `json:"text" redis:"text"`
	URL   string `json:"url" redis:"url"`
}

func (b BannerContent) MarshalBinary() ([]byte, error) {
	return json.Marshal(b)
}

func (b *BannerContent) UnmarshalBinary(data []byte) error {
	err := json.Unmarshal(data, &b)
	if err != nil {
		return err
	}
	return nil
}
