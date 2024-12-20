package service

import (
	"context"
	
	"github.com/purelind/check-tiup-nightly/internal/checker"
	"github.com/purelind/check-tiup-nightly/internal/database"
	"github.com/purelind/check-tiup-nightly/pkg/logger"
)

type Updater struct {
	db *database.DB
}

func NewUpdater(db *database.DB) *Updater {
	return &Updater{db: db}
}

func (u *Updater) UpdateAllComponentsCommits(ctx context.Context) error {
	components := []string{"tidb", "tikv", "pd", "tiflash"}

	for _, component := range components {
		if err := u.UpdateComponentCommit(ctx, component); err != nil {
			logger.Error("Failed to update commit info for", component, ":", err)
			continue
		}
	}
	return nil
}

func (u *Updater) UpdateComponentCommit(ctx context.Context, component string) error {
	info, err := checker.FetchLatestCommitInfo(ctx, component, "master")
	if err != nil {
		return err
	}

	if err := u.db.UpdateBranchCommit(ctx, info); err != nil {
		return err
	}

	logger.Info("Updated commit info for", component, info.Branch, ":", info.GitHash)
	return nil
}