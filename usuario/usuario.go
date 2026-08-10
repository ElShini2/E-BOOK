package usuario

import (
	"e-book/modelo"
	"errors"
)

// Constantes para definir los roles del sistema
const (
	RolAdmin   = "ADMIN"
	RolCliente = "CLIENTE"
)

// Usuario representa la entidad de un usuario (Admin o Cliente).
type Usuario struct {
	ID        int             `json:"id"`
	Nombre    string          `json:"nombre"`
	Email     string          `json:"email"`
	Password  string          `json:"-"` // No se exporta en la respuesta JSON por seguridad
	Rol       string          `json:"rol"`
	Favoritos []*modelo.Libro `json:"favoritos"`
}

// NewUsuario crea una nueva instancia de usuario validando sus datos.
func NewUsuario(id int, nombre, email, password, rol string) (*Usuario, error) {
	if nombre == "" || email == "" || password == "" {
		return nil, errors.New("el nombre, correo y contraseña no pueden estar vacíos")
	}

	if rol != RolAdmin && rol != RolCliente {
		rol = RolCliente // Rol por defecto si no es admin
	}

	return &Usuario{
		ID:        id,
		Nombre:    nombre,
		Email:     email,
		Password:  password,
		Rol:       rol,
		Favoritos: []*modelo.Libro{},
	}, nil
}

// ValidarPassword verifica si la contraseña ingresada coincide.
func (u *Usuario) ValidarPassword(pass string) bool {
	return u.Password == pass
}

// AgregarFavorito añade un libro a la lista de favoritos.
func (u *Usuario) AgregarFavorito(libro *modelo.Libro) error {
	if libro == nil {
		return errors.New("no se puede agregar un libro nulo")
	}

	for _, fav := range u.Favoritos {
		if fav.ID == libro.ID {
			return errors.New("el libro ya se encuentra en favoritos")
		}
	}

	u.Favoritos = append(u.Favoritos, libro)
	return nil
}

// ObtenerFavoritos retorna la lista de libros favoritos.
func (u *Usuario) ObtenerFavoritos() []*modelo.Libro {
	return u.Favoritos
}
