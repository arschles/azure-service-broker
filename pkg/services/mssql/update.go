package mssql

import (
	"context"
	"errors"
	"fmt"

	"github.com/Azure/azure-service-broker/pkg/service"
	uuid "github.com/satori/go.uuid"
)

func (m *module) ValidateUpdatingParameters(
	updatingParameters service.UpdatingParameters,
) error {
	return nil
}

func (m *module) GetUpdater(string, string) (service.Updater, error) {
	return service.NewUpdater(
		service.NewUpdatingStep("update", m.update),
	)
}

func (m *module) update(
	ctx context.Context, // nolint: unparam
	instanceID string, // nolint: unparam
	serviceID string,
	planID string,
	provisioningContext service.ProvisioningContext,
	updatingParameters service.UpdatingParameters,
) (service.ProvisioningContext, error) {
	pc, ok := provisioningContext.(*mssqlProvisioningContext)
	if !ok {
		return nil, errors.New(
			"error casting provisioningContext as *mssqlProvisioningContext",
		)
	}
	up, ok := updatingParameters.(*UpdatingParameters)
	if !ok {
		return nil, errors.New(
			"error casting updatingParameters as " +
				"*mssql.UpdatingParameters",
		)
	}

	// Update administrator
	if up.AdministratorLogin != "" {
		pc.AdministratorLogin = up.AdministratorLogin
	}
	if up.AdministratorLoginPassword != "" {
		pc.AdministratorLoginPassword = up.AdministratorLoginPassword
	}

	if planID == "" {
		return pc, nil
	}

	// Update service plan
	catalog, err := m.GetCatalog()
	if err != nil {
		return nil, fmt.Errorf("error retrieving catalog: %s", err)
	}
	service, ok := catalog.GetService(serviceID)
	if !ok {
		return nil, fmt.Errorf(
			`service "%s" not found in the "%s" module catalog`,
			serviceID,
			m.GetName(),
		)
	}
	plan, ok := service.GetPlan(planID)
	if !ok {
		return nil, fmt.Errorf(
			`plan "%s" not found for service "%s"`,
			planID,
			serviceID,
		)
	}

	pc.ARMDeploymentName = uuid.NewV4().String()
	if _, err := m.armDeployer.Deploy(
		pc.ARMDeploymentName,
		pc.ResourceGroupName,
		pc.Location,
		armTemplateExistingServerBytes,
		map[string]interface{}{
			"serverName":   pc.ServerName,
			"databaseName": pc.DatabaseName,
			"edition":      plan.GetProperties().Extended["edition"],
			"requestedServiceObjectiveName": plan.GetProperties().
				Extended["requestedServiceObjectiveName"],
			"maxSizeBytes": plan.GetProperties().
				Extended["maxSizeBytes"],
		},
		pc.Tags,
	); err != nil {
		return nil, fmt.Errorf("error deploying ARM template: %s", err)
	}

	return pc, nil
}
