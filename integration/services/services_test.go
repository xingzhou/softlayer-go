package services_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	datatypes "github.com/maximilien/softlayer-go/data_types"
	softlayer "github.com/maximilien/softlayer-go/softlayer"
	testhelpers "github.com/maximilien/softlayer-go/test_helpers"
)

var _ = Describe("SoftLayer Services", func() {
	var (
		err error

		accountService      softlayer.SoftLayer_Account_Service
		virtualGuestService softlayer.SoftLayer_Virtual_Guest_Service
	)

	BeforeEach(func() {
		accountService, err = testhelpers.CreateAccountService()
		Expect(err).ToNot(HaveOccurred())
		Expect(accountService).ToNot(BeNil())

		virtualGuestService, err = testhelpers.CreateVirtualGuestService()
		Expect(err).ToNot(HaveOccurred())
		Expect(virtualGuestService).ToNot(BeNil())
	})

	Context("uses SoftLayer_Account to list current virtual: disk images, guests, ssh keys, and network storage", func() {
		It("returns an array of SoftLayer_Virtual_Guest disk images", func() {
			virtualDiskImages, err := accountService.GetVirtualDiskImages()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(virtualDiskImages)).To(BeNumerically(">=", 0))
		})

		It("returns an array of SoftLayer_Virtual_Guest objects", func() {
			virtualGuests, err := accountService.GetVirtualGuests()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(virtualGuests)).To(BeNumerically(">=", 0))
		})

		It("returns an array of SoftLayer_Virtual_Guest network storage", func() {
			networkStorageArray, err := accountService.GetNetworkStorage()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(networkStorageArray)).To(BeNumerically(">=", 0))
		})

		It("returns an array of SoftLayer_Ssh_Keys objects", func() {
			sshKeys, err := accountService.GetSshKeys()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(sshKeys)).To(BeNumerically(">=", 0))
		})
	})

	Context("uses SoftLayer_Account to create and then delete a an ssh key", func() {
		BeforeEach(func() {
			err := testhelpers.FindAndDeleteTestSshKeys()
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := testhelpers.FindAndDeleteTestSshKeys()
			Expect(err).ToNot(HaveOccurred())
		})

		It("creates the ssh key and verify it is present and then deletes it", func() {
			sshKeyPath := os.Getenv("SOFTLAYER_GO_TEST_SSH_KEY_PATH")
			Expect(sshKeyPath).ToNot(Equal(""), "SOFTLAYER_GO_TEST_SSH_KEY_PATH env variable is not set")

			testSshKeyValue, err := ioutil.ReadFile(sshKeyPath)
			Expect(err).ToNot(HaveOccurred())

			sshKey := datatypes.SoftLayer_Security_Ssh_Key{
				Key:   strings.Trim(string(testSshKeyValue), "\n"),
				Label: testhelpers.TEST_LABEL_PREFIX,
				Notes: testhelpers.TEST_NOTES_PREFIX,
			}

			sshKeyService, err := testhelpers.CreateSecuritySshKeyService()
			Expect(err).ToNot(HaveOccurred())

			//Create ssh key
			createdSshKey, err := sshKeyService.CreateObject(sshKey)
			Expect(err).ToNot(HaveOccurred())
			Expect(createdSshKey.Key).To(Equal(sshKey.Key), "key")
			Expect(createdSshKey.Label).To(Equal(sshKey.Label), "label")
			Expect(createdSshKey.Notes).To(Equal(sshKey.Notes), "notes")
			Expect(createdSshKey.CreateDate).ToNot(BeNil(), "createDate")
			Expect(createdSshKey.Fingerprint).ToNot(Equal(""), "fingerprint")
			Expect(createdSshKey.Id).To(BeNumerically(">", 0), "id")
			Expect(createdSshKey.ModifyDate).To(BeNil(), "modifyDate")

			//Delete ssh key
			deleted, err := sshKeyService.DeleteObject(createdSshKey.Id)
			Expect(err).ToNot(HaveOccurred())
			Expect(deleted).To(BeTrue())
		})
	})

	Context("uses SoftLayer_Account to create and then delete a virtual guest instance", func() {
		var (
			TIMEOUT          time.Duration
			POLLING_INTERVAL time.Duration
		)

		BeforeEach(func() {
			TIMEOUT = 5 * time.Minute
			POLLING_INTERVAL = 10 * time.Second

			err := testhelpers.FindAndDeleteTestVirtualGuests()
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := testhelpers.FindAndDeleteTestVirtualGuests()
			Expect(err).ToNot(HaveOccurred())
		})

		It("creates the virtual guest instance and waits for it to be active then delete it", func() {
			virtualGuestTemplate := datatypes.SoftLayer_Virtual_Guest_Template{
				Hostname:  "test",
				Domain:    "softlayergo.com",
				StartCpus: 1,
				MaxMemory: 1024,
				Datacenter: datatypes.Datacenter{
					Name: "ams01",
				},
				HourlyBillingFlag:            true,
				LocalDiskFlag:                true,
				OperatingSystemReferenceCode: "UBUNTU_LATEST",
			}

			virtualGuestService, err := testhelpers.CreateVirtualGuestService()
			Expect(err).ToNot(HaveOccurred())

			fmt.Printf("----> creating new virtual guest\n")
			virtualGuest, err := virtualGuestService.CreateObject(virtualGuestTemplate)
			Expect(err).ToNot(HaveOccurred())
			fmt.Printf("----> created virtual guest: %d\n", virtualGuest.Id)

			fmt.Printf("----> waiting for virtual guest: %d, until RUNNING\n", virtualGuest.Id)
			Eventually(func() string {
				vgPowerState, err := virtualGuestService.GetPowerState(virtualGuest.Id)
				Expect(err).ToNot(HaveOccurred())
				fmt.Printf("----> virtual guest: %d, has power state: %s\n", virtualGuest.Id, vgPowerState.KeyName)
				return vgPowerState.KeyName
			}, TIMEOUT, POLLING_INTERVAL).Should(Equal("RUNNING"), "failed waiting for virtual guest to be RUNNING")

			fmt.Printf("----> marking virtual guest with TEST:softlayer-go\n")
			err = testhelpers.MarkVirtualGuestAsTest(virtualGuest)
			Expect(err).ToNot(HaveOccurred(), "Could not mark virtual guest as test")
			fmt.Printf("----> marked virtual guest with TEST:softlayer-go\n")

			fmt.Printf("----> waiting for virtual guest to have no active transactions pending\n")
			Eventually(func() int {
				activeTransactions, err := virtualGuestService.GetActiveTransactions(virtualGuest.Id)
				Expect(err).ToNot(HaveOccurred())
				fmt.Printf("----> virtual guest: %d, has %d active transactions\n", virtualGuest.Id, len(activeTransactions))
				return len(activeTransactions)
			}, TIMEOUT, POLLING_INTERVAL).Should(Equal(0), "failed waiting for virtual guest to have no active transactions")

			fmt.Printf("----> deleting virtual guest: %d\n", virtualGuest.Id)
			deleted, err := virtualGuestService.DeleteObject(virtualGuest.Id)
			Expect(err).ToNot(HaveOccurred(), "Error deleting virtual guest")
			Expect(deleted).To(BeTrue())
		})
	})

	XContext("uses SoftLayer_Account to create a new instance and network storage and attach them", func() {
		It("creates the virtual guest instance and waits for it to be active", func() {
			Expect(false).To(BeTrue())
		})

		It("creates the disk storage and attaches it to the instance", func() {
			Expect(false).To(BeTrue())
		})

		It("deletes the virtual guest instance if it is running", func() {
			Expect(false).To(BeTrue())
		})

		It("detaches and deletes the network storage if available", func() {
			Expect(false).To(BeTrue())
		})
	})
})
