package entity

type GetUserBannerDTO struct {
	TagID           int64
	FeatureID       int64
	UseLastRevision bool
	IsAdmin         bool
}

type GetBannersDTO struct {
	Filters map[string]int64
	Limit   int
	Offset  int
}

type CreateBannerDTO struct {
	TagIDs    []int64       `json:"tag_ids"`
	FeatureID int64         `json:"feature_id"`
	Content   BannerContent `json:"content"`
	IsActive  bool          `json:"is_active"`
}

type UpdateBannerDTO struct {
	BannerID  int64
	TagIDs    []int64       `json:"tag_ids"`
	FeatureID int64         `json:"feature_id"`
	Content   BannerContent `json:"content"`
	IsActive  bool          `json:"is_active"`
}

type DeleteBannerDTO struct {
	BannerID int64
}

type UpdateCacheDTO struct {
	BannerID  int64
	Content   BannerContent
	TagID     int64
	FeatureID int64
	IsActive  bool
	IsAdmin   bool
}
