package service

import "github.com/rabb1tof/socialsentry/backend/internal/domain"

// PlanLimits captures the resource caps and platform allowances of a subscription plan.
// A value of -1 means "unlimited".
type PlanLimits struct {
	MaxAccounts           int
	MaxTriggersPerAccount int
	LogRetentionDays      int
	AllowedPlatforms      []string
	MultiplePlatforms     bool // basic = one platform at a time; pro/enterprise = any mix
}

// PlanLimitsByName returns the limits for a plan name. Unknown names default to basic.
func PlanLimitsByName(plan string) PlanLimits {
	switch plan {
	case domain.PlanPro:
		return PlanLimits{
			MaxAccounts:           10,
			MaxTriggersPerAccount: 50,
			LogRetentionDays:      30,
			AllowedPlatforms:      []string{domain.PlatformVK, domain.PlatformInstagram},
			MultiplePlatforms:     true,
		}
	case domain.PlanEnterprise:
		return PlanLimits{
			MaxAccounts:           -1,
			MaxTriggersPerAccount: -1,
			LogRetentionDays:      90,
			AllowedPlatforms:      []string{domain.PlatformVK, domain.PlatformInstagram},
			MultiplePlatforms:     true,
		}
	default: // basic
		return PlanLimits{
			MaxAccounts:           2,
			MaxTriggersPerAccount: 5,
			LogRetentionDays:      7,
			AllowedPlatforms:      []string{domain.PlatformVK, domain.PlatformInstagram},
			MultiplePlatforms:     false,
		}
	}
}
