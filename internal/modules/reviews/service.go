package reviews

import (
	"fmt"
	"time"

	"gogin/internal/clients"
	"gogin/internal/models"

	"github.com/google/uuid"
)

type ReviewsService struct {
	db *clients.Database
}

func NewReviewsService(db *clients.Database) *ReviewsService {
	return &ReviewsService{db: db}
}

func (s *ReviewsService) CreateReview(userID string, req *CreateReviewRequest) (*ReviewResponse, error) {
	id := uuid.New().String()
	query := `
		INSERT INTO reviews (id, resource_type, resource_id, user_id, rating, title, content, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
		RETURNING created_at, updated_at
	`

	var createdAt, updatedAt time.Time
	err := s.db.QueryRow(query, id, req.ResourceType, req.ResourceID, userID, req.Rating, req.Title, req.Content, "published").Scan(&createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create review: %w", err)
	}

	return &ReviewResponse{
		ID:           id,
		ResourceType: req.ResourceType,
		ResourceID:   req.ResourceID,
		UserID:       userID,
		Rating:       req.Rating,
		Title:        req.Title,
		Content:      req.Content,
		Status:       "published",
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}, nil
}

func (s *ReviewsService) ListReviews(resourceType, resourceID string, page, limit int) ([]*ReviewResponse, int, float64, error) {
	offset := (page - 1) * limit

	var total int
	var avgRating float64
	err := s.db.QueryRow(`SELECT COUNT(*), COALESCE(AVG(rating), 0) FROM reviews WHERE resource_type = $1 AND resource_id = $2 AND status = 'published'`, resourceType, resourceID).Scan(&total, &avgRating)
	if err != nil {
		return nil, 0, 0, err
	}

	query := `SELECT id, resource_type, resource_id, user_id, rating, title, content, status, created_at, updated_at FROM reviews WHERE resource_type = $1 AND resource_id = $2 AND status = 'published' ORDER BY created_at DESC LIMIT $3 OFFSET $4`
	rows, err := s.db.Query(query, resourceType, resourceID, limit, offset)
	if err != nil {
		return nil, 0, 0, err
	}
	defer rows.Close()

	var reviews []*ReviewResponse
	for rows.Next() {
		var r models.Review
		var title string
		rows.Scan(&r.ID, &r.ResourceType, &r.ResourceID, &r.UserID, &r.Rating, &r.Title, &r.Content, &r.Status, &r.CreatedAt, &r.UpdatedAt)
		if r.Title.Valid {
			title = r.Title.String
		}
		reviews = append(reviews, &ReviewResponse{r.ID, r.ResourceType, r.ResourceID, r.UserID, r.Rating, title, r.Content, r.Status, r.CreatedAt, r.UpdatedAt})
	}

	return reviews, total, avgRating, nil
}

func (s *ReviewsService) GetReview(id string) (*ReviewResponse, error) {
	var r models.Review
	err := s.db.QueryRow(`SELECT id, resource_type, resource_id, user_id, rating, title, content, status, created_at, updated_at FROM reviews WHERE id = $1`, id).Scan(&r.ID, &r.ResourceType, &r.ResourceID, &r.UserID, &r.Rating, &r.Title, &r.Content, &r.Status, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	title := ""
	if r.Title.Valid {
		title = r.Title.String
	}
	return &ReviewResponse{r.ID, r.ResourceType, r.ResourceID, r.UserID, r.Rating, title, r.Content, r.Status, r.CreatedAt, r.UpdatedAt}, nil
}

func (s *ReviewsService) UpdateReview(id, userID string, req *UpdateReviewRequest) (*ReviewResponse, error) {
	result, err := s.db.Exec(`UPDATE reviews SET rating = $1, title = $2, content = $3, updated_at = NOW() WHERE id = $4 AND user_id = $5`, req.Rating, req.Title, req.Content, id, userID)
	if err != nil {
		return nil, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return nil, fmt.Errorf("review not found")
	}
	return s.GetReview(id)
}

func (s *ReviewsService) DeleteReview(id, userID string) error {
	result, err := s.db.Exec(`DELETE FROM reviews WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return fmt.Errorf("review not found")
	}
	return nil
}
