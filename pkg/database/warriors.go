package database

import (
	"database/sql"
	"errors"
	"log"
)

// GetRegisteredUsers retrieves the registered warriors from db
func (d *Database) GetRegisteredUsers(Limit int, Offset int) []*Warrior {
	var warriors = make([]*Warrior, 0)
	rows, err := d.db.Query(
		`
		SELECT id, name, email, rank, avatar, verified
		FROM warriors
		WHERE email IS NOT NULL
		ORDER BY created_date
		LIMIT $1
		OFFSET $2
		`,
		Limit,
		Offset,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var w Warrior
			var warriorEmail sql.NullString

			if err := rows.Scan(
				&w.WarriorID,
				&w.WarriorName,
				&warriorEmail,
				&w.WarriorRank,
				&w.WarriorAvatar,
				&w.Verified,
			); err != nil {
				log.Println(err)
			} else {
				w.WarriorEmail = warriorEmail.String
				warriors = append(warriors, &w)
			}
		}
	} else {
		log.Println(err)
	}

	return warriors
}

// GetUser gets a warrior from db by ID
func (d *Database) GetUser(WarriorID string) (*Warrior, error) {
	var w Warrior
	var warriorEmail sql.NullString

	e := d.db.QueryRow(
		"SELECT id, name, email, rank, avatar, verified, notifications_enabled FROM warriors WHERE id = $1",
		WarriorID,
	).Scan(
		&w.WarriorID,
		&w.WarriorName,
		&warriorEmail,
		&w.WarriorRank,
		&w.WarriorAvatar,
		&w.Verified,
		&w.NotificationsEnabled,
	)
	if e != nil {
		log.Println(e)
		return nil, errors.New("warrior not found")
	}

	w.WarriorEmail = warriorEmail.String

	return &w, nil
}

func (d *Database) GetUserByEmail(WarriorEmail string) (*Warrior, error) {
	var w Warrior
	e := d.db.QueryRow(
		"SELECT id, name, email, rank, verified FROM warriors WHERE email = $1",
		WarriorEmail,
	).Scan(
		&w.WarriorID,
		&w.WarriorName,
		&w.WarriorEmail,
		&w.WarriorRank,
		&w.Verified,
	)
	if e != nil {
		log.Println(e)
		return nil, errors.New("warrior email not found")
	}

	return &w, nil
}

// AuthUser attempts to authenticate the warrior
func (d *Database) AuthUser(WarriorEmail string, WarriorPassword string) (*Warrior, error) {
	var w Warrior
	var passHash string

	e := d.db.QueryRow(
		`SELECT id, name, email, rank, password, avatar, verified, notifications_enabled FROM warriors WHERE email = $1`,
		WarriorEmail,
	).Scan(
		&w.WarriorID,
		&w.WarriorName,
		&w.WarriorEmail,
		&w.WarriorRank,
		&passHash,
		&w.WarriorAvatar,
		&w.Verified,
		&w.NotificationsEnabled,
	)
	if e != nil {
		log.Println(e)
		return nil, errors.New("warrior not found")
	}

	if !ComparePasswords(passHash, []byte(WarriorPassword)) {
		return nil, errors.New("password invalid")
	}

	return &w, nil
}

// CreateUserGuest adds a new warrior private (guest) to the db
func (d *Database) CreateUserGuest(WarriorName string) (*Warrior, error) {
	var WarriorID string
	e := d.db.QueryRow(`INSERT INTO warriors (name) VALUES ($1) RETURNING id`, WarriorName).Scan(&WarriorID)
	if e != nil {
		log.Println(e)
		return nil, errors.New("unable to create new warrior")
	}

	return &Warrior{WarriorID: WarriorID, WarriorName: WarriorName, WarriorAvatar: "identicon", NotificationsEnabled: true}, nil
}

// CreateUserRegistered adds a new warrior corporal (registered) to the db
func (d *Database) CreateUserRegistered(WarriorName string, WarriorEmail string, WarriorPassword string, ActiveWarriorID string) (NewWarrior *Warrior, VerifyID string, RegisterErr error) {
	hashedPassword, hashErr := HashAndSalt([]byte(WarriorPassword))
	if hashErr != nil {
		return nil, "", hashErr
	}

	var WarriorID string
	var verifyID string
	WarriorRank := "CORPORAL"
	WarriorAvatar := "identicon"

	if ActiveWarriorID != "" {
		e := d.db.QueryRow(
			`SELECT userId, verifyId FROM register_existing_user($1, $2, $3, $4, $5);`,
			ActiveWarriorID,
			WarriorName,
			WarriorEmail,
			hashedPassword,
			WarriorRank,
		).Scan(&WarriorID, &verifyID)
		if e != nil {
			log.Println(e)
			return nil, "", errors.New("a warrior with that email already exists")
		}
	} else {
		e := d.db.QueryRow(
			`SELECT userId, verifyId FROM register_user($1, $2, $3, $4);`,
			WarriorName,
			WarriorEmail,
			hashedPassword,
			WarriorRank,
		).Scan(&WarriorID, &verifyID)
		if e != nil {
			log.Println(e)
			return nil, "", errors.New("a warrior with that email already exists")
		}
	}

	return &Warrior{WarriorID: WarriorID, WarriorName: WarriorName, WarriorEmail: WarriorEmail, WarriorRank: WarriorRank, WarriorAvatar: WarriorAvatar}, verifyID, nil
}

// UpdateUserProfile attempts to update the warriors profile
func (d *Database) UpdateUserProfile(WarriorID string, WarriorName string, WarriorAvatar string, NotificationsEnabled bool) error {
	if WarriorAvatar == "" {
		WarriorAvatar = "identicon"
	}
	if _, err := d.db.Exec(
		`UPDATE warriors SET name = $2, avatar = $3, notifications_enabled=$4 WHERE id = $1;`,
		WarriorID,
		WarriorName,
		WarriorAvatar,
		NotificationsEnabled,
	); err != nil {
		log.Println(err)
		return errors.New("error attempting to update warriors profile")
	}

	return nil
}

// UserResetRequest inserts a new warrior reset request
func (d *Database) UserResetRequest(WarriorEmail string) (resetID string, warriorName string, resetErr error) {
	var ResetID sql.NullString
	var WarriorID sql.NullString
	var WarriorName sql.NullString

	e := d.db.QueryRow(`
		SELECT resetId, userId, warriorName FROM insert_user_reset($1);
		`,
		WarriorEmail,
	).Scan(&ResetID, &WarriorID, &WarriorName)
	if e != nil {
		log.Println("Unable to reset warrior: ", e)
		return "", "", e
	}

	return ResetID.String, WarriorName.String, nil
}

// UserResetPassword attempts to reset a warriors password
func (d *Database) UserResetPassword(ResetID string, WarriorPassword string) (warriorName string, warriorEmail string, resetErr error) {
	var WarriorName sql.NullString
	var WarriorEmail sql.NullString

	hashedPassword, hashErr := HashAndSalt([]byte(WarriorPassword))
	if hashErr != nil {
		return "", "", hashErr
	}

	warErr := d.db.QueryRow(`
		SELECT
			w.name, w.email
		FROM warrior_reset wr
		LEFT JOIN warriors w ON w.id = wr.warrior_id
		WHERE wr.reset_id = $1;
		`,
		ResetID,
	).Scan(&WarriorName, &WarriorEmail)
	if warErr != nil {
		log.Println("Unable to get warrior for password reset confirmation email: ", warErr)
		return "", "", warErr
	}

	if _, err := d.db.Exec(
		`call reset_user_password($1, $2)`, ResetID, hashedPassword); err != nil {
		return "", "", err
	}

	return WarriorName.String, WarriorEmail.String, nil
}

// UserUpdatePassword attempts to update a warriors password
func (d *Database) UserUpdatePassword(WarriorID string, WarriorPassword string) (warriorName string, warriorEmail string, resetErr error) {
	var WarriorName sql.NullString
	var WarriorEmail sql.NullString

	warErr := d.db.QueryRow(`
		SELECT
			w.name, w.email
		FROM warriors w
		WHERE w.id = $1;
		`,
		WarriorID,
	).Scan(&WarriorName, &WarriorEmail)
	if warErr != nil {
		log.Println("Unable to get warrior for password update: ", warErr)
		return "", "", warErr
	}

	hashedPassword, hashErr := HashAndSalt([]byte(WarriorPassword))
	if hashErr != nil {
		return "", "", hashErr
	}

	if _, err := d.db.Exec(
		`call update_user_password($1, $2)`, WarriorID, hashedPassword); err != nil {
		return "", "", err
	}

	return WarriorName.String, WarriorEmail.String, nil
}

// VerifyUserAccount attempts to verify a warriors account email
func (d *Database) VerifyUserAccount(VerifyID string) error {
	if _, err := d.db.Exec(
		`call verify_user_account($1)`, VerifyID); err != nil {
		return err
	}

	return nil
}

// UpdateUserProfile attempts to delete a warrior
func (d *Database) DeleteUser(WarriorID string) error {
	if _, err := d.db.Exec(
		`call delete_user($1);`,
		WarriorID,
	); err != nil {
		log.Println(err)
		return errors.New("error attempting to delete warrior")
	}

	return nil
}
