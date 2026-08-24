package handlers

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"

	"github.com/riont/crm/backend/internal/repository"
)

var validTemplateCategories = map[string]bool{
	"marketing": true,
	"utility":   true,
	"soporte":   true,
	"general":   true,
}

type TemplatesHandler struct {
	repo *repository.TemplateRepository
}

func NewTemplatesHandler(repo *repository.TemplateRepository) *TemplatesHandler {
	return &TemplatesHandler{repo: repo}
}

type TemplateResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Category  string `json:"category"`
	Content   string `json:"content"`
	Language  string `json:"language"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func newTemplateResponse(t *repository.Template) *TemplateResponse {
	return &TemplateResponse{
		ID:        t.ID.String(),
		Name:      t.Name,
		Category:  t.Category,
		Content:   t.Content,
		Language:  t.Language,
		CreatedAt: t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: t.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// validateTemplateFields checks name/content/category/language when present.
func validateTemplateFields(name, category, content, language *string) error {
	if category != nil {
		*category = strings.TrimSpace(*category)
		if *category == "" {
			return errors.New("'category' no puede estar vacío")
		}
		if !validTemplateCategories[*category] {
			return errors.New("'category' debe ser 'marketing', 'utility', 'soporte' o 'general'")
		}
	}
	if language != nil {
		*language = strings.TrimSpace(*language)
		if utf8.RuneCountInString(*language) != 2 {
			return errors.New("'language' debe ser un código de 2 caracteres")
		}
	}
	if name != nil {
		n := strings.TrimSpace(*name)
		if l := utf8.RuneCountInString(n); l < 2 || l > 80 {
			return errors.New("'name' debe tener entre 2 y 80 caracteres")
		}
		*name = n
	}
	if content != nil {
		ct := strings.TrimSpace(*content)
		if ct == "" {
			return errors.New("'content' no puede estar vacío")
		}
		if utf8.RuneCountInString(ct) > 2000 {
			return errors.New("'content' debe tener como máximo 2000 caracteres")
		}
		*content = ct
	}
	return nil
}

// ListTemplates returns templates with optional ?search= and ?category= filters.
// GET /api/templates
func (h *TemplatesHandler) ListTemplates(c echo.Context) error {
	if h.repo == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "database not connected"})
	}

	templates, err := h.repo.List(c.Request().Context(), c.QueryParam("search"), c.QueryParam("category"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al listar las plantillas"})
	}

	results := []*TemplateResponse{}
	for _, t := range templates {
		results = append(results, newTemplateResponse(t))
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data":  results,
		"count": len(results),
	})
}

// CreateTemplate creates a template from {name, category?, content, language?}.
// POST /api/templates
func (h *TemplatesHandler) CreateTemplate(c echo.Context) error {
	if h.repo == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "database not connected"})
	}

	var body struct {
		Name     *string `json:"name"`
		Category *string `json:"category"`
		Content  *string `json:"content"`
		Language *string `json:"language"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "cuerpo de la petición inválido"})
	}

	category := "general"
	if body.Category != nil {
		category = *body.Category
	}
	language := "es"
	if body.Language != nil {
		language = *body.Language
	}

	name := ""
	if body.Name != nil {
		name = *body.Name
	}
	content := ""
	if body.Content != nil {
		content = *body.Content
	}

	if err := validateTemplateFields(&name, &category, &content, &language); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	t, err := h.repo.Create(c.Request().Context(), name, category, content, language)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al crear la plantilla"})
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{"data": newTemplateResponse(t)})
}

// UpdateTemplate applies partial changes to a template.
// PUT /api/templates/:id
func (h *TemplatesHandler) UpdateTemplate(c echo.Context) error {
	if h.repo == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "database not connected"})
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "id inválido"})
	}

	var body struct {
		Name     *string `json:"name"`
		Category *string `json:"category"`
		Content  *string `json:"content"`
		Language *string `json:"language"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "cuerpo de la petición inválido"})
	}

	if err := validateTemplateFields(body.Name, body.Category, body.Content, body.Language); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	t, err := h.repo.Update(c.Request().Context(), id, body.Name, body.Category, body.Content, body.Language)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "plantilla no encontrada"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al actualizar la plantilla"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"data": newTemplateResponse(t)})
}

// DeleteTemplate removes a template by id.
// DELETE /api/templates/:id
func (h *TemplatesHandler) DeleteTemplate(c echo.Context) error {
	if h.repo == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "database not connected"})
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "id inválido"})
	}

	if err := h.repo.Delete(c.Request().Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "plantilla no encontrada"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error al eliminar la plantilla"})
	}

	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}
