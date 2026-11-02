package http

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/felipinho/me-lab/internal/catalog"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleListProfiles(w http.ResponseWriter, r *http.Request) {
	RespondJSON(w, http.StatusOK, s.catalog.Profiles())
}

func (s *Server) handleListBlocksCatalog(w http.ResponseWriter, r *http.Request) {
	profileID := strings.TrimSpace(r.URL.Query().Get("profile_id"))
	if profileID == "" {
		profileID = catalog.DefaultProfileID
	}
	if _, ok := s.catalog.GetProfile(profileID); !ok {
		RespondError(w, r, http.StatusNotFound, ErrCodeNotFound, "Compatibility profile not found")
		return
	}
	RespondJSON(w, http.StatusOK, s.catalog.BlocksForProfile(profileID))
}

func (s *Server) handleGetBlockDefinition(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "blockDefinitionId")
	blockID, err := url.PathUnescape(rawID)
	if err != nil || strings.TrimSpace(blockID) == "" {
		RespondError(w, r, http.StatusBadRequest, ErrCodeBadRequest, "Invalid block definition ID")
		return
	}

	profileID := strings.TrimSpace(r.URL.Query().Get("profile_id"))
	if profileID == "" {
		profileID = catalog.DefaultProfileID
	}

	def, ok := s.catalog.GetBlock(profileID, blockID)
	if !ok {
		RespondError(w, r, http.StatusNotFound, ErrCodeNotFound, "Block definition not found")
		return
	}
	RespondJSON(w, http.StatusOK, def)
}
