package images

import (
	"database/sql"

	"app/internal"
	"app/internal/db"
)

// ListImagesQuery picks one page of a folder's images. Without a status it lists every status.
// q keeps the images whose path, relative to the folder, contains it, ignoring ASCII case; the
// matches are then listed by path, and sort does not apply. sort saved is for status optimized.
type ListImagesQuery struct {
	Status string `form:"status" json:"status" binding:"omitempty,oneof=pending optimizing optimized kept skipped failed gone"`
	Search string `form:"q" json:"q" binding:"max=200"`
	Sort   string `form:"sort" json:"sort" binding:"omitempty,oneof=updated saved"`
	Cursor string `form:"cursor" json:"cursor"`
	Limit  int64  `form:"limit" json:"limit" binding:"omitempty,min=1,max=100"`
}

// DailySavingsQuery picks one folder's savings, or every folder's together without folder_id.
type DailySavingsQuery struct {
	FolderID int64 `form:"folder_id" json:"folder_id" binding:"omitempty,min=1"`
}

// ReasonResponse is a stored reason read back. code is null for text that starts with no code this
// version knows, and detail is then all of it. detail is what was measured, in English, such as
// "0.7100, below 0.8500, at 128,64".
type ReasonResponse struct {
	Code   *string `json:"code" extensions:"x-nullable" enums:"symlink,not_regular_file,hard_link,access_control_list,wrong_content,apng,extra_data,too_many_pixels,too_large,unreadable,already_optimized,too_new,changed_while_optimizing,interrupted,tool_missing,tool_failed,swap_failed,no_saving,format_changed,result_undecodable,dimensions_changed,frames_changed,animation_changed,pixels_changed,ssim_below_threshold,color_changed,metadata_changed"`
	Detail *string `json:"detail" extensions:"x-nullable"`
} //	@name	ReasonResponse

// ImageResponse is one image of a folder. size is the file on disk now; original_size, for an
// optimized image, what it was before. reason says why it is as it is, and lossless_reason, after
// a lossy result was refused, how the lossless attempt that followed ended. next_attempt_at is
// when a failed image is tried again, null once it has failed for good.
type ImageResponse struct {
	ID             int64           `json:"id"`
	FolderID       int64           `json:"folder_id"`
	Path           string          `json:"path"`
	Format         string          `json:"format" enums:"png,jpeg,gif"`
	Status         string          `json:"status" enums:"pending,optimizing,optimized,kept,skipped,failed,gone"`
	Size           int64           `json:"size"`
	OriginalSize   *int64          `json:"original_size" extensions:"x-nullable"`
	Reason         *ReasonResponse `json:"reason" extensions:"x-nullable"`
	LosslessReason *ReasonResponse `json:"lossless_reason" extensions:"x-nullable"`
	Attempts       int64           `json:"attempts"`
	NextAttemptAt  *string         `json:"next_attempt_at" extensions:"x-nullable"`
	UpdatedAt      string          `json:"updated_at"`
} //	@name	ImageResponse

// ImagesPage is one page of images and the cursor of the next, null on the last page. order is
// the order they are in: updated (the latest change first), saved (the most bytes saved first) or
// path (a search's matches).
type ImagesPage struct {
	Items      []ImageResponse `json:"items"`
	NextCursor *string         `json:"next_cursor" extensions:"x-nullable"`
	Order      string          `json:"order" enums:"updated,saved,path"`
}

// ImagesPageResponse wraps a page of images
type ImagesPageResponse struct {
	Data ImagesPage `json:"data"`
}

// ImageDataResponse wraps an image
type ImageDataResponse struct {
	Data ImageResponse `json:"data"`
}

// RetriedResponse is how many failed images went back to pending
type RetriedResponse struct {
	Data RetriedData `json:"data"`
}

type RetriedData struct {
	Retried int64 `json:"retried"`
}

// DaySavingsResponse is one UTC day of optimizations: how many, and the bytes they saved.
type DaySavingsResponse struct {
	Day        string `json:"day"`
	Optimized  int64  `json:"optimized"`
	BytesSaved int64  `json:"bytes_saved"`
}

// DailySavingsResponse is the last 30 UTC days, oldest first, today last, a day without
// optimizations as zero.
type DailySavingsResponse struct {
	Data []DaySavingsResponse `json:"data"`
}

func imageResponse(image db.Image) ImageResponse {
	response := ImageResponse{
		ID:            image.ID,
		FolderID:      image.FolderID,
		Path:          image.Name,
		Format:        image.Format,
		Status:        image.Status,
		Size:          image.Size,
		Attempts:      image.Attempts,
		NextAttemptAt: internal.FormatOptionalTime(image.NextAttemptAt),
		UpdatedAt:     internal.FormatTime(sql.NullTime{Time: image.UpdatedAt, Valid: true}),
	}
	if image.Dir != "" {
		response.Path = image.Dir + "/" + image.Name
	}
	if Status(image.Status) == StatusOptimized && image.OriginalSize.Valid {
		response.OriginalSize = &image.OriginalSize.Int64
	}
	if image.Reason.Valid {
		reason, lossless := ParseReason(image.Reason.String)
		response.Reason = reasonResponse(reason)
		if lossless != nil {
			response.LosslessReason = reasonResponse(*lossless)
		}
	}
	return response
}

func reasonResponse(r Reason) *ReasonResponse {
	response := &ReasonResponse{}
	if r.Code != "" {
		response.Code = &r.Code
	}
	if r.Detail != "" {
		response.Detail = &r.Detail
	}
	return response
}
