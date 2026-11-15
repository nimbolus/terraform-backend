package server

// Migration of states stored before v0.3.0. Their ID was ambiguous (see terraform.GetLegacyStateID) and they were
// stored without metadata. A lock moves a state to its new ID and format.
// TODO: remove in 1.0.0.

import (
	"errors"
	"fmt"
	"io"

	log "github.com/sirupsen/logrus"

	"github.com/nimbolus/terraform-backend/pkg/storage"
	"github.com/nimbolus/terraform-backend/pkg/terraform"
)

func readLegacyState(state *terraform.State, store storage.Storage) ([]byte, error) {
	stored, err := store.GetState(state.LegacyID)
	if err != nil {
		return nil, err
	}

	log.Infof("read state with id %s from legacy id %s, the next lock migrates it", state.ID, state.LegacyID)

	return stored.Data, nil
}

// migrateLegacyState moves the state from its legacy ID to the new ID and format. It must only be called while the
// request holds the lock of the state, so no write can happen in between. A failure is only logged, the state is then
// still read from its legacy ID.
func migrateLegacyState(state *terraform.State, store storage.Storage) {
	if state.LegacyID == "" {
		return
	}

	// The legacy ID first: after the migration it doesn't exist, so a lock doesn't read the whole state.
	legacy, err := store.GetState(state.LegacyID)
	if errors.Is(err, storage.ErrStateNotFound) {
		return
	} else if err != nil {
		log.Errorf("failed to read legacy id %s of state with id %s for migration: %v", state.LegacyID, state.ID, err)
		return
	}

	existing, err := store.GetState(state.ID)
	if err == nil {
		// Already migrated, a legacy copy left over would only be stale. Unless the new state is broken, then the
		// legacy copy may be the only intact one.
		if _, err := terraform.UnmarshalStoredState(existing.Data, state); err != nil {
			log.Errorf("keeping legacy id %s, the state with id %s is invalid: %v", state.LegacyID, state.ID, err)
			return
		}

		_ = deleteLegacyState(state, store)
		return
	} else if !errors.Is(err, storage.ErrStateNotFound) {
		log.Errorf("failed to check state with id %s for migration: %v", state.ID, err)
		return
	}

	// the legacy data is encrypted already
	migrated := &terraform.State{ID: state.ID, Project: state.Project, Name: state.Name, Data: legacy.Data}
	if err := saveState(migrated, store); err != nil {
		log.Errorf("failed to migrate state with id %s from legacy id %s: %v", state.ID, state.LegacyID, err)
		return
	}

	_ = deleteLegacyState(state, store)

	log.Infof("migrated state with id %s from legacy id %s", state.ID, state.LegacyID)
}

// deleteLegacyState deletes the legacy copy of the state. A failure is logged and returned.
func deleteLegacyState(state *terraform.State, store storage.Storage) error {
	if state.LegacyID == "" {
		return nil
	}

	if err := store.DeleteState(state.LegacyID); err != nil && !errors.Is(err, storage.ErrStateNotFound) {
		log.Errorf("failed to delete legacy copy %s of state with id %s: %v", state.LegacyID, state.ID, err)
		return fmt.Errorf("deleting legacy copy: %w", err)
	}

	return nil
}

// CheckMigration lists the states that are still stored with a legacy ID and returns their number. A storage without any
// state is an error.
func CheckMigration(store storage.Storage, w io.Writer) (int, error) {
	lister, ok := store.(storage.Lister)
	if !ok {
		return 0, fmt.Errorf("storage backend %s can't list states", store.GetName())
	}

	ids, err := lister.ListStateIDs()
	if err != nil {
		return 0, fmt.Errorf("listing states: %w", err)
	}

	// a wrong storage configuration, such as a container without the server's volume, finds no states, which must not
	// pass as migrated
	if len(ids) == 0 {
		return 0, fmt.Errorf("found no states in the %s storage, check that it has the server's storage configuration",
			store.GetName())
	}

	legacy := 0

	for _, id := range ids {
		if terraform.IsLegacyStateID(id) {
			legacy++

			if _, err := fmt.Fprintf(w, "not migrated: %s\n", id); err != nil {
				return 0, err
			}
		}
	}

	if _, err := fmt.Fprintf(w, "%d of %d states not migrated\n", legacy, len(ids)); err != nil {
		return 0, err
	}

	return legacy, nil
}
