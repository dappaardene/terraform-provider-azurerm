package iothub

//go:generate go run ../../tools/generator-resource-id/main.go -path=./ -name=Enrichment -id=/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/resGroup1/providers/Microsoft.Devices/IotHubs/hub1/Enrichments/enrichment1
//go:generate go run ../../tools/generator-resource-id/main.go -path=./ -name=IotHub -id=/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/resGroup1/providers/Microsoft.Devices/IotHubs/hub1
//go:generate go run ../../tools/generator-resource-id/main.go -path=./ -name=IotHubDps -id=/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/resGroup1/providers/Microsoft.Devices/provisioningServices/provisioningService1
//go:generate go run ../../tools/generator-resource-id/main.go -path=./ -name=DpsCertificate -id=/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/resGroup1/providers/Microsoft.Devices/provisioningServices/provisioningService1/certificates/certificate1
//go:generate go run ../../tools/generator-resource-id/main.go -path=./ -name=DpsSharedAccessPolicy -id=/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/resGroup1/providers/Microsoft.Devices/provisioningServices/provisioningService1/keys/sharedAccessPolicy1
