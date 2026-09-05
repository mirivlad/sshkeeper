package cmd

import (
	"github.com/mirivlad/sshkeeper/internal/model"
	"github.com/mirivlad/sshkeeper/internal/ssh"
)

func dbProfileResolver(serverID int64) (*model.Server, error) {
	return appDB.GetServerByID(serverID)
}

func parseRouteSpec(spec string) (model.Route, error) {
	return model.ParseRouteSpec(spec, appDB.ResolveAlias)
}

func serverVaultFunc(server *model.Server) ssh.VaultFunc {
	return vaultFuncForServer(getOrCreateVault(), server)
}

func rollbackSavedServer(server, original *model.Server) {
	if original != nil {
		_ = appDB.UpdateServerByAlias(server.Alias, original)
		_ = appDB.SetServerTags(original.ID, original.Tags)
		return
	}
	_ = appDB.DeleteServer(server.Alias)
}
