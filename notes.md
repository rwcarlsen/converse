
Data layout:

* Each user can have a "conversations" directory in a standard location in
  their upspin tree.

* Each conversation thread is its own directory in the main "conversations"
  directory.

* Each message in a conversation is a file.  The message files contain the
  message author, message timestamp, and file name of the previous/parent
  message (not subnamed by revision mod # - just the root message num)

* Each message/reply in the conversation is a file named "msg[num]-[user].txt"

* Edits/modifications to a message are "msg1.1-[user].txt" for the 1st
  modified revision version of message 1, "msg1.2-[user].txt" for the 2nd
  modified revision and so on.

* Each participant in the conversation always tries to retrieve the latest
  messages before publishing a new message and assigning it a message
  number.  Conflicts are okay - if two people post a message with the same
  number, then the timestamps and parent message file names will be used to
  resolve the order by the various user interfaces.

* File contents must be cryptographically signed by the sender (and verified
  by the receiver).  Where should the signature live?

Inviting People to a conversation:

* When someone is invited to the conversation, they the inviter creates a new
  folder in the invitee's conversations directory and also creates a file in
  that directory containing the names of all conversation participant(s).

* When someone receives an invitation (a newly created conversations dir) to
  participate in a new conversation, they will need to grant read access to
  all participants to the conversation's directory in their upspin tree.

* Participants in a conversation are controlled by the upspin Access
  file.  Access can be controlled, revoked, etc. at any time by just the
  normal upspin Access control methods.

