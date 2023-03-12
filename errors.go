package trackerclient

type TrackerError string

func (e TrackerError) Error() string { return string(e) }

const ErrNoTasksAvailable = TrackerError("no tasks available")

const ErrInvalidTrackerResponse = TrackerError("invalid tracker response")

const ErrNoSuchProject = TrackerError("this project doesn't exist")
