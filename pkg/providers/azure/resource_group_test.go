package azure

// // MockResourceGroupsClient is a mock of the Azure SDK ResourceGroupsClient
// type MockResourceGroupsClient struct {
// 	mock.Mock
// }

// func TestCreateResourceGroup(t *testing.T) {
// 	// Setup
// 	viper.Set("azure.resource_prefix", "testRG")
// 	location := "eastus"

// 	t.Run("Create new resource group", func(t *testing.T) {
// 		mockClient := GetMockAzureClient().(*MockAzureClient)
// 		ctx := context.Background()

// 		// Act
// 		mockClient.GetOrCreateResourceGroupFunc = func(ctx context.Context,
// 			location string,
// 			name string,
// 			tags map[string]*string) (*armresources.ResourceGroup, error) {
// 			return &armresources.ResourceGroup{
// 				Location: &location,
// 			}, nil
// 		}

// 		result, err := mockClient.GetOrCreateResourceGroupFunc(ctx, location, "TESTRGNAME", nil)

// 		// Assert
// 		assert.NoError(t, err)
// 		assert.NotNil(t, result)
// 		assert.Equal(t, location, *result.Location)
// 	})

// 	t.Run("Resource group already exists", func(t *testing.T) {
// 		mockClient := GetMockAzureClient().(*MockAzureClient)
// 		ctx := context.Background()

// 		// Act
// 		mockClient.GetOrCreateResourceGroupFunc = func(ctx context.Context,
// 			location string,
// 			name string,
// 			tags map[string]*string) (*armresources.ResourceGroup, error) {
// 			return &armresources.ResourceGroup{
// 				Location: &location,
// 			}, nil
// 		}
// 		result, err := mockClient.GetOrCreateResourceGroupFunc(ctx, location, "TESTRGNAME", nil)

// 		// Assert
// 		assert.NoError(t, err)
// 		assert.NotNil(t, result)
// 		assert.Equal(t, location, *result.Location)
// 	})

// 	t.Run("Create resource group fails", func(t *testing.T) {
// 		mockClient := GetMockAzureClient().(*MockAzureClient)
// 		ctx := context.Background()

// 		// Act
// 		mockClient.GetOrCreateResourceGroupFunc = func(ctx context.Context, location string, name string, tags map[string]*string) (*armresources.ResourceGroup, error) {
// 			return nil, errors.New("error")
// 		}
// 		result, err := mockClient.GetOrCreateResourceGroupFunc(ctx, location, "TESTRGNAME", nil)

// 		// Assert
// 		assert.Error(t, err)
// 		assert.Nil(t, result)
// 	})
// }
