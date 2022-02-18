package main

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

func (bot *robot) handleMember(expectRepo expectRepoInfo, localMembers []string, repoOwner *string, log *logrus.Entry) []string {
	org := expectRepo.org
	repo := expectRepo.getNewRepoName()

	if len(localMembers) == 0 {
		v, err := bot.cli.GetRepo(org, repo)
		if err != nil {
			log.Errorf("handle repo members and get repo:%s, err:%s", repo, err.Error())
			return nil
		}
		localMembers = toLowerOfMembers(v.Members)
		*repoOwner = v.Owner.Login
	}

	expect := sets.NewString(expectRepo.expectOwners...)
	lm := sets.NewString(localMembers...)
	r := expect.Intersection(lm).UnsortedList()

	allCollaborators, err := bot.cli.ListCollaborators(org, repo)
	if err != nil {
		log.Errorf("list all collaborators failed, err: %v", err)
		return nil
	}

	localAdmins := make([]string, 0)
	for _, item := range allCollaborators {
		if item.Permissions.Admin == true {
			localAdmins = append(localAdmins, strings.ToLower(item.Login))
		}
	}

	expectAdmins := toLowerOfMembers(expectRepo.expectAdmins)
	ea := sets.NewString(expectAdmins...)
	la := sets.NewString(localAdmins...)

	// add new
	if v := expect.Difference(lm); v.Len() > 0 {
		for k := range v {
			l := log.WithField("add member", fmt.Sprintf("%s:%s", repo, k))
			l.Info("start")

			// how about adding a member but he/she exits? see the comment of 'addRepoMember'
			if err := bot.addRepoMember(org, repo, k); err != nil {
				l.Error(err)
			} else {
				r = append(r, k)
			}
		}
	}

	// remove
	if v := lm.Difference(expect); v.Len() > 0 {
		o := *repoOwner

		for k := range v {
			if k == o {
				// Gitee does not allow to remove the repo owner.
				continue
			}

			l := log.WithField("remove member", fmt.Sprintf("%s:%s", repo, k))
			l.Info("start")

			if err := bot.cli.RemoveRepoMember(org, repo, k); err != nil {
				l.Error(err)

				r = append(r, k)
			}
		}
	}

	//add admins
	if v := ea.Difference(la); v.Len() > 0 {
		for k := range v {
			if !expect.Has(k) {
				continue
			}

			if expect.Has(k) {

				l := log.WithField("update developer to admin", fmt.Sprintf("%s:%s", repo, k))
				l.Info("start")

				if err := bot.cli.RemoveRepoMember(org, repo, k); err != nil {
					l.Errorf("remove developer %s from %s/%s failed, err: %v", k, org, repo, err)
				}

				if err := bot.addRepoAdmin(org, repo, k); err != nil {
					l.Errorf("add admin %s to %s/%s failed, err: %v", k, org, repo, err)
				}
			}
		}
	}

	//remove admins
	if v := la.Difference(ea); v.Len() > 0 {
		o := *repoOwner

		if o == "" {
			v, err := bot.cli.GetRepo(org, repo)
			if err != nil {
				log.Errorf("handle repo members and get repo:%s, err:%s", repo, err.Error())
				return nil
			}
			*repoOwner = v.Owner.Login
			o = *repoOwner
		}

		for k := range v {

			if k == o {
				continue
			}

			if expect.Has(k) {

				l := log.WithField("update admin to developer", fmt.Sprintf("%s:%s", repo, k))
				l.Info("start")

				if err := bot.cli.RemoveRepoMember(org, repo, k); err != nil {
					l.Errorf("remove admin %s from %s/%s failed, err: %v", k, org, repo, err)
				}

				if err := bot.addRepoMember(org, repo, k); err != nil {
					l.Errorf("add developer %s to %s/%s failed, err: %v", k, org, repo, err)
				}
			}
		}
	}

	return r
}

// Gitee api will be successful even if adding a member repeatedly.
func (bot *robot) addRepoMember(org, repo, login string) error {
	return bot.cli.AddRepoMember(org, repo, login, "push")
}

func (bot *robot) addRepoAdmin(org, repo, login string) error {
	return bot.cli.AddRepoMember(org, repo, login, "admin")
}

func toLowerOfMembers(m []string) []string {
	v := make([]string, len(m))
	for i := range m {
		v[i] = strings.ToLower(m[i])
	}
	return v
}
