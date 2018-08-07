package commands

type NewIdentityCmd struct{}

func (cmd *NewIdentityCmd) Execute(args []string) error {
	/*
		id, err := identity.New()
		if err != nil {
			return err
		}
		err = config.SaveUserIdentity(id, cmd.Args.Email)
		if err != nil {
			return err
		}

		signature := id.SignMessage([]byte(cmd.Args.Token))

		id, err := idFromString(email)
		if err != nil {
			return nil, err
		}
		uuk := v1.UpdateUserKey{
			PublicKey: hex.EncodeToString(id.Public.Key[:]),
		}

		var uukr v1.UpdateUserKeyReply
		err = c.Post(v1.RouteUpdateUserKey, uuk, &uukr)
		if err != nil {
			return nil, err
		}

		sig := id.SignMessage([]byte(uukr.VerificationToken))
		vuuk := v1.VerifyUpdateUserKey{
			VerificationToken: uukr.VerificationToken,
			Signature:         hex.EncodeToString(sig[:]),
		}

		responseBody, err = c.makeRequest("POST", v1.RouteVerifyUpdateUserKey, vuuk)
		if err != nil {
			return nil, err
		}

		var vuukr v1.VerifyUpdateUserKeyReply
		err = json.Unmarshal(responseBody, &vuukr)
		if err != nil {
			return nil, fmt.Errorf("Could not unmarshal VerifyUpdateUserKeyReply: %v",
				err)
		}

		if Cfg.Verbose {
			prettyPrintJSON(vuukr)
		}

		return id, nil
	*/
	return nil
}
