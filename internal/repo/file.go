package model

type File struct {
	Id         int    `json:"id" gorm:"primaryKey;autoIncrement"`
	ObjectId   string `json:"object_id" gorm:"column:object_id"`
	Object     string `json:"object"`
	Bytes      int    `json:"bytes"`
	Filename   string `json:"filename"`
	Purpose    string `json:"purpose"`
	Tokens     int    `json:"tokens" gorm:"default:0"`
	UserId     int    `json:"user_id" gorm:"not null;index"`
	CreatedAt  int64  `json:"created_at" gorm:"autoCreateTime"`
	UploadedAt int64  `json:"uploaded_at" gorm:"autoUpdateTime"`
}

func (f *File) Insert() error {
	return DB.Create(f).Error
}

func DeleteFile(objectId string) error {
	return DB.Where("object_id = ?", objectId).Delete(&File{}).Error
}

func ListFile(userId int) ([]File, error) {
	var files []File
	err := DB.Find(&files, "user_id = ?", userId).Error
	return files, err
}

func GetFileByObjectId(objectId string, userId int) (*File, error) {
	var file File
	err := DB.Where("object_id = ? and user_id = ?", objectId, userId).First(&file).Error
	return &file, err
}
