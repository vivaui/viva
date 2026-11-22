//go:build linux

package vulkan

import (
	"testing"
	"unsafe"
)

func TestVulkanStructureABI(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Fatal("only 64-bit platforms are supported")
	}
	for _, layout := range []struct {
		name string
		size uintptr
		want uintptr
	}{
		{"VkApplicationInfo", unsafe.Sizeof(vkApplicationInfo{}), 48},
		{"VkInstanceCreateInfo", unsafe.Sizeof(vkInstanceCreateInfo{}), 64},
		{"VkQueueFamilyProperties", unsafe.Sizeof(vkQueueFamilyProperties{}), 24},
		{"VkDeviceQueueCreateInfo", unsafe.Sizeof(vkDeviceQueueCreateInfo{}), 40},
		{"VkDeviceCreateInfo", unsafe.Sizeof(vkDeviceCreateInfo{}), 72},
		{"VkSurfaceCapabilitiesKHR", unsafe.Sizeof(vkSurfaceCapabilities{}), 52},
		{"VkSwapchainCreateInfoKHR", unsafe.Sizeof(vkSwapchainCreateInfo{}), 104},
		{"VkImageViewCreateInfo", unsafe.Sizeof(vkImageViewCreateInfo{}), 80},
		{"VkAttachmentDescription", unsafe.Sizeof(vkAttachmentDescription{}), 36},
		{"VkSubpassDescription", unsafe.Sizeof(vkSubpassDescription{}), 72},
		{"VkRenderPassCreateInfo", unsafe.Sizeof(vkRenderPassCreateInfo{}), 64},
		{"VkFramebufferCreateInfo", unsafe.Sizeof(vkFramebufferCreateInfo{}), 64},
		{"VkCommandPoolCreateInfo", unsafe.Sizeof(vkCommandPoolCreateInfo{}), 24},
		{"VkCommandBufferAllocateInfo", unsafe.Sizeof(vkCommandBufferAllocateInfo{}), 32},
		{"VkCommandBufferBeginInfo", unsafe.Sizeof(vkCommandBufferBeginInfo{}), 32},
		{"VkClearAttachment", unsafe.Sizeof(vkClearAttachment{}), 24},
		{"VkClearRect", unsafe.Sizeof(vkClearRect{}), 24},
		{"VkRenderPassBeginInfo", unsafe.Sizeof(vkRenderPassBeginInfo{}), 64},
		{"VkSemaphoreCreateInfo", unsafe.Sizeof(vkSemaphoreCreateInfo{}), 24},
		{"VkSubmitInfo", unsafe.Sizeof(vkSubmitInfo{}), 72},
		{"VkPresentInfoKHR", unsafe.Sizeof(vkPresentInfo{}), 64},
		{"VkPhysicalDeviceMemoryProperties", unsafe.Sizeof(vkPhysicalDeviceMemoryProperties{}), 520},
		{"VkMemoryRequirements", unsafe.Sizeof(vkMemoryRequirements{}), 24},
		{"VkBufferCreateInfo", unsafe.Sizeof(vkBufferCreateInfo{}), 56},
		{"VkMemoryAllocateInfo", unsafe.Sizeof(vkMemoryAllocateInfo{}), 32},
		{"VkShaderModuleCreateInfo", unsafe.Sizeof(vkShaderModuleCreateInfo{}), 40},
		{"VkPipelineShaderStageCreateInfo", unsafe.Sizeof(vkPipelineShaderStageCreateInfo{}), 48},
		{"VkVertexInputBindingDescription", unsafe.Sizeof(vkVertexInputBindingDescription{}), 12},
		{"VkVertexInputAttributeDescription", unsafe.Sizeof(vkVertexInputAttributeDescription{}), 16},
		{"VkPipelineVertexInputStateCreateInfo", unsafe.Sizeof(vkPipelineVertexInputStateCreateInfo{}), 48},
		{"VkPipelineInputAssemblyStateCreateInfo", unsafe.Sizeof(vkPipelineInputAssemblyStateCreateInfo{}), 32},
		{"VkPipelineViewportStateCreateInfo", unsafe.Sizeof(vkPipelineViewportStateCreateInfo{}), 48},
		{"VkPipelineRasterizationStateCreateInfo", unsafe.Sizeof(vkPipelineRasterizationStateCreateInfo{}), 64},
		{"VkPipelineMultisampleStateCreateInfo", unsafe.Sizeof(vkPipelineMultisampleStateCreateInfo{}), 48},
		{"VkPipelineColorBlendAttachmentState", unsafe.Sizeof(vkPipelineColorBlendAttachmentState{}), 32},
		{"VkPipelineColorBlendStateCreateInfo", unsafe.Sizeof(vkPipelineColorBlendStateCreateInfo{}), 56},
		{"VkPipelineDynamicStateCreateInfo", unsafe.Sizeof(vkPipelineDynamicStateCreateInfo{}), 32},
		{"VkPushConstantRange", unsafe.Sizeof(vkPushConstantRange{}), 12},
		{"VkPipelineLayoutCreateInfo", unsafe.Sizeof(vkPipelineLayoutCreateInfo{}), 48},
		{"VkGraphicsPipelineCreateInfo", unsafe.Sizeof(vkGraphicsPipelineCreateInfo{}), 144},
		{"VkViewport", unsafe.Sizeof(vkViewport{}), 24},
		{"VkExtent3D", unsafe.Sizeof(vkExtent3D{}), 12},
		{"VkImageCreateInfo", unsafe.Sizeof(vkImageCreateInfo{}), 88},
		{"VkSamplerCreateInfo", unsafe.Sizeof(vkSamplerCreateInfo{}), 80},
		{"VkDescriptorSetLayoutBinding", unsafe.Sizeof(vkDescriptorSetLayoutBinding{}), 24},
		{"VkDescriptorSetLayoutCreateInfo", unsafe.Sizeof(vkDescriptorSetLayoutCreateInfo{}), 32},
		{"VkDescriptorPoolSize", unsafe.Sizeof(vkDescriptorPoolSize{}), 8},
		{"VkDescriptorPoolCreateInfo", unsafe.Sizeof(vkDescriptorPoolCreateInfo{}), 40},
		{"VkDescriptorSetAllocateInfo", unsafe.Sizeof(vkDescriptorSetAllocateInfo{}), 40},
		{"VkDescriptorImageInfo", unsafe.Sizeof(vkDescriptorImageInfo{}), 24},
		{"VkWriteDescriptorSet", unsafe.Sizeof(vkWriteDescriptorSet{}), 64},
		{"VkImageMemoryBarrier", unsafe.Sizeof(vkImageMemoryBarrier{}), 72},
		{"VkImageSubresourceLayers", unsafe.Sizeof(vkImageSubresourceLayers{}), 16},
		{"VkBufferImageCopy", unsafe.Sizeof(vkBufferImageCopy{}), 56},
		{"scene instance", unsafe.Sizeof(vulkanRectangleInstance{}), 80},
	} {
		if layout.size != layout.want {
			t.Errorf("%s size %d, want %d", layout.name, layout.size, layout.want)
		}
	}
	if offset := unsafe.Offsetof(vkImageCreateInfo{}.QueueFamilies); offset != 72 {
		t.Errorf("VkImageCreateInfo.pQueueFamilyIndices offset %d, want 72", offset)
	}
	if offset := unsafe.Offsetof(vkImageCreateInfo{}.InitialLayout); offset != 80 {
		t.Errorf("VkImageCreateInfo.initialLayout offset %d, want 80", offset)
	}
}
