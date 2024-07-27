func destroyAllDeployments(cmd *cobra.Command) error {
	l := logger.Get()
	l.Info("Destroying all deployments")

	azureProvider, err := azure.AzureProviderFunc(viper.GetViper())
	if err != nil {
		return fmt.Errorf("failed to initialize Azure provider: %w", err)
	}

	deployments, err := azureProvider.ListDeployments(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to list deployments: %w", err)
	}

	for _, deployment := range deployments {
		l.Infof("Starting destruction of deployment: %s", deployment.Name)
		err := azureProvider.DestroyDeployment(cmd.Context(), deployment.Name)
		if err != nil {
			l.Errorf("Failed to destroy deployment %s: %v", deployment.Name, err)
		} else {
			l.Infof("Successfully destroyed deployment: %s", deployment.Name)
		}
	}

	l.Info("All deployments have been processed")
	return nil
}
