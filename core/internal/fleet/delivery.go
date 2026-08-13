package fleet

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"autosecrets.dev/core/internal/database"
	"autosecrets.dev/core/internal/envelope"
	"autosecrets.dev/core/internal/middleware"
	"filippo.io/age"
)

// desiredResponse is what a node receives on polling: a stable ETag plus one
// encrypted, signed envelope per assigned Bundle Revision.
type desiredResponse struct {
	ETag      string               `json:"etag"`
	Envelopes []*envelope.Envelope `json:"envelopes"`
	Cleanup   []cleanupInstruction `json:"cleanup,omitempty"`
}

type cleanupInstruction struct {
	AssignmentID  string   `json:"assignment_id"`
	ApplicationID string   `json:"application_id"`
	EnvironmentID string   `json:"environment_id"`
	Units         []string `json:"units,omitempty"`
}

// handleDesired serves the node's complete Desired State.
func (h *Handler) handleDesired(w http.ResponseWriter, r *http.Request) {
	node, err := h.nodeFromRequest(r)
	if err != nil {
		middleware.WriteError(w, http.StatusForbidden, "forbidden", "unknown node identity")
		return
	}
	revisionIDs, err := h.store.AssignedRevisions(r.Context(), node.ID)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	cleanup := h.cleanupFor(r, node)
	cleanupIDs := make([]string, 0, len(cleanup))
	for _, instruction := range cleanup {
		cleanupIDs = append(cleanupIDs, instruction.AssignmentID)
	}
	etag := etagOf(append(revisionIDs, cleanupIDs...))
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	_ = h.store.SetNodeDesired(r.Context(), node.ID, etag)
	if len(revisionIDs) == 0 {
		middleware.WriteJSON(w, http.StatusOK, desiredResponse{ETag: etag, Envelopes: []*envelope.Envelope{}, Cleanup: cleanup})
		return
	}
	recipient, err := age.ParseX25519Recipient(node.AgePubkey)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	envs := make([]*envelope.Envelope, 0, len(revisionIDs))
	for _, revisionID := range revisionIDs {
		env, err := h.buildEnvelope(r, node, revisionID, recipient)
		if err != nil {
			middleware.WriteError(w, http.StatusInternalServerError, "internal", "internal error")
			return
		}
		envs = append(envs, env)
	}
	middleware.WriteJSON(w, http.StatusOK, desiredResponse{ETag: etag, Envelopes: envs, Cleanup: cleanup})
}

func (h *Handler) cleanupFor(r *http.Request, node *database.Node) []cleanupInstruction {
	instructions, err := h.store.PendingCleanupInstructions(r.Context(), node.ID)
	if err != nil {
		return nil
	}
	out := make([]cleanupInstruction, 0, len(instructions))
	for _, instruction := range instructions {
		out = append(out, cleanupInstruction{
			AssignmentID:  instruction.AssignmentID,
			ApplicationID: instruction.ApplicationID,
			EnvironmentID: instruction.EnvironmentID,
			Units:         instruction.Units,
		})
	}
	return out
}

func (h *Handler) buildEnvelope(r *http.Request, node *database.Node, revisionID string, recipient *age.X25519Recipient) (*envelope.Envelope, error) {
	files, err := h.store.RevisionFiles(r.Context(), revisionID)
	if err != nil {
		return nil, err
	}
	appID, envID, err := h.store.RevisionAppEnv(r.Context(), revisionID)
	if err != nil {
		return nil, err
	}
	type payloadFile struct {
		Path    string `json:"path"`
		Mode    string `json:"mode"`
		UID     string `json:"uid"`
		GID     string `json:"gid"`
		Content string `json:"content"`
	}
	payload := struct {
		AppID string        `json:"app_id"`
		EnvID string        `json:"env_id"`
		Files []payloadFile `json:"files"`
	}{AppID: appID, EnvID: envID, Files: []payloadFile{}}
	manifestFiles := []envelope.FileSpec{}
	for _, f := range files {
		wrapped, nonces, ct, err := h.store.SecretVersionBlob(r.Context(), f.SecretID, f.VersionSeq)
		if err != nil {
			return nil, err
		}
		plain, err := h.mk.Open(wrapped, nonces, ct)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(plain)
		manifestFiles = append(manifestFiles, envelope.FileSpec{
			Path:   f.Path,
			Mode:   middleware.ModeString(f.Mode),
			UID:    strconv.FormatInt(f.UID, 10),
			GID:    strconv.FormatInt(f.GID, 10),
			SHA256: hex.EncodeToString(sum[:]),
		})
		payload.Files = append(payload.Files, payloadFile{
			Path:    f.Path,
			Mode:    middleware.ModeString(f.Mode),
			UID:     strconv.FormatInt(f.UID, 10),
			GID:     strconv.FormatInt(f.GID, 10),
			Content: base64.StdEncoding.EncodeToString(plain),
		})
	}
	manifestBytes, err := envelope.CanonicalManifest(manifestFiles)
	if err != nil {
		return nil, err
	}
	plaintextBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return envelope.New(envelope.Options{
		NodeID:       node.ID,
		RevisionID:   revisionID,
		CreatedAt:    h.now(),
		ExpiresAt:    h.now().Add(10 * time.Minute),
		Manifest:     manifestBytes,
		Plaintext:    plaintextBytes,
		Recipient:    recipient,
		Signer:       h.signer.PrivateKey(),
		SigningKeyID: h.signer.KeyID(),
	})
}

func (h *Handler) nodeFromRequest(r *http.Request) (*database.Node, error) {
	serial, ok := middleware.AgentSerialFromContext(r.Context())
	if !ok {
		return nil, errors.New("no node identity")
	}
	return h.store.NodeBySerial(r.Context(), serial)
}

func (h *Handler) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	node, err := h.nodeFromRequest(r)
	if err != nil {
		middleware.WriteError(w, http.StatusForbidden, "forbidden", "unknown node identity")
		return
	}
	if node.ID != r.PathValue("nodeID") {
		middleware.WriteError(w, http.StatusForbidden, "forbidden", "node identity mismatch")
		return
	}
	if err := h.store.TouchNode(r.Context(), node.ID, node.ObservedRevision, node.LastResult, h.now()); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleReport(w http.ResponseWriter, r *http.Request) {
	node, err := h.nodeFromRequest(r)
	if err != nil {
		middleware.WriteError(w, http.StatusForbidden, "forbidden", "unknown node identity")
		return
	}
	if node.ID != r.PathValue("nodeID") {
		middleware.WriteError(w, http.StatusForbidden, "forbidden", "node identity mismatch")
		return
	}
	var body struct {
		RevisionID string `json:"revision_id"`
		Stage      string `json:"stage"`
		Result     string `json:"result"`
		Error      string `json:"error,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Result == "" {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "result is required")
		return
	}
	lastResult := body.Result
	if body.Error != "" {
		lastResult += ": " + body.Error
	}
	asg, asgErr := h.store.AssignmentForNodeRevision(r.Context(), node.ID, body.RevisionID)
	if asgErr == nil {
		reportedAt := h.now()
		if err := h.store.RecordConvergence(r.Context(), database.ConvergenceRow{
			NodeID: node.ID, AssignmentID: asg.ID,
			ApplicationID: asg.ApplicationID, EnvironmentID: asg.EnvironmentID,
			DesiredRevision: asg.RevisionID, ObservedRevision: body.RevisionID,
			Stage: body.Stage, Result: body.Result, Error: body.Error,
			ReportedAt: &reportedAt,
		}); err != nil {
			middleware.WriteError(w, http.StatusInternalServerError, "internal", "internal error")
			return
		}
	}
	if err := h.store.TouchNode(r.Context(), node.ID, body.RevisionID, lastResult, h.now()); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	_ = h.store.AppendAudit(r.Context(), nil, database.AuditEvent{
		Actor: "node:" + node.Name, Action: "activation." + body.Stage, Resource: body.RevisionID,
		Result: body.Result, CorrelationID: middleware.CorrelationID(h.now, r),
	})
	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "recorded"})
}

func (h *Handler) handleCleanupReport(w http.ResponseWriter, r *http.Request) {
	node, err := h.nodeFromRequest(r)
	if err != nil {
		middleware.WriteError(w, http.StatusForbidden, "forbidden", "unknown node identity")
		return
	}
	if node.ID != r.PathValue("nodeID") {
		middleware.WriteError(w, http.StatusForbidden, "forbidden", "node identity mismatch")
		return
	}
	var body struct {
		AssignmentID string `json:"assignment_id"`
		Result       string `json:"result"`
		Error        string `json:"error,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.AssignmentID == "" || body.Result == "" {
		middleware.WriteError(w, http.StatusBadRequest, "bad_request", "assignment_id and result are required")
		return
	}
	if err := h.store.ReportCleanupTask(r.Context(), body.AssignmentID, node.ID, body.Result, body.Error); err != nil {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "no pending cleanup task for this node")
		return
	}
	_ = h.store.AppendAudit(r.Context(), nil, database.AuditEvent{
		Actor: "node:" + node.Name, Action: "assignment.cleanup", Resource: body.AssignmentID,
		Result: body.Result, CorrelationID: middleware.CorrelationID(h.now, r),
	})
	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "recorded"})
}

func etagOf(revisionIDs []string) string {
	sorted := append([]string(nil), revisionIDs...)
	sort.Strings(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, ",")))
	return `"` + hex.EncodeToString(sum[:]) + `"`
}
