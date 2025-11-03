package reviews

import "time"

// CreateReviewRequest represents a review creation request
type CreateReviewRequest struct {
	ResourceType string `json:"resource_type" binding:"required"`
	ResourceID   string `json:"resource_id" binding:"required"`
	Rating       int    `json:"rating" binding:"required,min=1,max=5"`
	Title        string `json:"title" binding:"required"`
	Content      string `json:"content" binding:"required"`
}

// UpdateReviewRequest represents a review update request
type UpdateReviewRequest struct {
	Rating  int    `json:"rating" binding:"required,min=1,max=5"`
	Title   string `json:"title" binding:"required"`
	Content string `json:"content" binding:"required"`
}

// ReviewResponse represents a review response
type ReviewResponse struct {
	ID           string    `json:"id"`
	ResourceType string    `json:"resource_type"`
	ResourceID   string    `json:"resource_id"`
	UserID       string    `json:"user_id"`
	Rating       int       `json:"rating"`
	Title        string    `json:"title"`
	Content      string    `json:"content"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ReviewsListResponse represents a paginated list of reviews
type ReviewsListResponse struct {
	Reviews      []*ReviewResponse `json:"reviews"`
	Total        int               `json:"total"`
	AverageRating float64          `json:"average_rating"`
	Page         int               `json:"page"`
	Limit        int               `json:"limit"`
	TotalPages   int               `json:"total_pages"`
}
