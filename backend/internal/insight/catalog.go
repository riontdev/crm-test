package insight

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CatalogItemRow es una fila de product_catalog con sus timestamps. No embebe
// CatalogItem a proposito: los dos tienen Aliases y el shadowing silencioso
// (gana el de fuera) hace muy facil escanear el campo equivocado.
type CatalogItemRow struct {
	// El id es lo que hace posible editar o borrar el producto desde la UI.
	// Sin el en la respuesta, /catalogo es una lista de solo lectura.
	ID        uuid.UUID `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	Category  string    `db:"category" json:"category"`
	Aliases   []string  `db:"aliases" json:"aliases"`
	Active    bool      `db:"active" json:"active"`
	SortOrder int       `db:"sort_order" json:"sort_order"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// CatalogPatch es el PATCH parcial de la UI. nil = no lo toques.
type CatalogPatch struct {
	Name      *string
	Category  *string
	Aliases   *[]string
	Active    *bool
	SortOrder *int
}

// CatalogRepository accede a product_catalog.
//
// Vive en el mismo Repository que el resto, pero con sus propios metodos, para
// no ensuciar la tabla de pedidos con CRUD de un catalogo que esread-mostly
// (se escribe cuando el operador lo edita, se lee en cada mensaje).
type CatalogRepository struct {
	repo *Repository
}

func NewCatalogRepository(repo *Repository) *CatalogRepository {
	return &CatalogRepository{repo: repo}
}

const catalogColumns = `id, name, category, aliases, active, sort_order, created_at, updated_at`

// List devuelve el catalogo. onlyActive=false lo usa el admin para ver tambien
// lo desactivado.
func (c *CatalogRepository) List(ctx context.Context, onlyActive bool) ([]CatalogItemRow, error) {
	query := `SELECT ` + catalogColumns + ` FROM product_catalog`
	if onlyActive {
		query += ` WHERE active = true`
	}
	query += ` ORDER BY category, sort_order, name`

	rows, err := c.repo.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list catalog: %w", err)
	}
	defer rows.Close()

	out := []CatalogItemRow{}
	for rows.Next() {
		var it CatalogItemRow
		if err := rows.Scan(&it.ID, &it.Name, &it.Category, &it.Aliases, &it.Active,
			&it.SortOrder, &it.CreatedAt, &it.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan catalog item: %w", err)
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate catalog: %w", err)
	}
	return out, nil
}

// PromptItems devuelve el catalogo como lo necesita el prompt. Es una copia
// acotada a MaxCatalogItems para que ningun caller pueda reventar el contexto.
func (c *CatalogRepository) PromptItems(ctx context.Context) ([]CatalogItem, error) {
	items, err := c.List(ctx, true)
	if err != nil {
		return nil, err
	}
	out := make([]CatalogItem, 0, len(items))
	for _, it := range items {
		out = append(out, CatalogItem{Name: it.Name, Category: it.Category, Aliases: it.Aliases})
		if len(out) >= MaxCatalogItems {
			break
		}
	}
	return out, nil
}

// Create agrega un producto. El nombre es UNIQUE: un duplicado es error de la
// UI, no algo que se resuelva silenciosamente.
func (c *CatalogRepository) Create(ctx context.Context, it CatalogItem, active bool, sortOrder int) (*CatalogItemRow, error) {
	name := strings.TrimSpace(it.Name)
	if name == "" {
		return nil, fmt.Errorf("el nombre del producto es obligatorio")
	}
	aliases := cleanAliases(it.Aliases)
	category := strings.TrimSpace(it.Category)
	if category == "" {
		category = "general"
	}

	var out CatalogItemRow
	err := c.repo.pool.QueryRow(ctx,
		`INSERT INTO product_catalog (name, category, aliases, active, sort_order)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING `+catalogColumns,
		name, category, aliases, active, sortOrder,
	).Scan(&out.ID, &out.Name, &out.Category, &out.Aliases, &out.Active,
		&out.SortOrder, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create catalog item: %w", err)
	}
	return &out, nil
}

// Update edita un producto.
func (c *CatalogRepository) Update(ctx context.Context, id uuid.UUID, p CatalogPatch) (*CatalogItemRow, error) {
	if p.Name != nil {
		n := strings.TrimSpace(*p.Name)
		if n == "" {
			return nil, fmt.Errorf("el nombre del producto es obligatorio")
		}
		p.Name = &n
	}
	if p.Category != nil {
		cat := strings.TrimSpace(*p.Category)
		if cat == "" {
			cat = "general"
		}
		p.Category = &cat
	}
	if p.Aliases != nil {
		cleaned := cleanAliases(*p.Aliases)
		p.Aliases = &cleaned
	}

	var out CatalogItemRow
	err := c.repo.pool.QueryRow(ctx,
		`UPDATE product_catalog
		 SET name       = COALESCE($2, name),
		     category   = COALESCE($3, category),
		     aliases    = COALESCE($4, aliases),
		     active     = COALESCE($5, active),
		     sort_order = COALESCE($6, sort_order),
		     updated_at = now()
		 WHERE id = $1
		 RETURNING `+catalogColumns,
		id, p.Name, p.Category, p.Aliases, p.Active, p.SortOrder,
	).Scan(&out.ID, &out.Name, &out.Category, &out.Aliases, &out.Active,
		&out.SortOrder, &out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update catalog item: %w", err)
	}
	return &out, nil
}

// Delete saca un producto del catalogo.
//
// Es borrado fisico a proposito: si queda la fila apagada, el operador ve un
// producto fantasma entre los 45 y no sabe si esta desactivado o roto.
func (c *CatalogRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := c.repo.pool.Exec(ctx, `DELETE FROM product_catalog WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete catalog item: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// cleanAliases saca vacios, repetidos y basura de mayusculas/acentos. Un alias
// con espacios raros nunca matchea por palabra completa y solo infla el prompt.
func cleanAliases(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, a := range in {
		a = strings.ToLower(strings.TrimSpace(a))
		if a == "" || len([]rune(a)) < 2 {
			continue
		}
		if seen[a] {
			continue
		}
		seen[a] = true
		out = append(out, a)
		if len(out) >= 20 {
			break
		}
	}
	return out
}
