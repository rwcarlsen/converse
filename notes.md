
* Each user can have a "conversations" directory in a standard location in
  their upspin tree.

* Each conversation thread is its own directory in the main "conversations"
  directory.

* Each message/reply in the conversation is a file named "msg[num].txt",
  "msg[num]-[user].txt", and so on.

  - Edits/modifications to a message are "msg1.1-[user].txt" for the 1st modified
    revision version of message 1, "msg1.2-[user].txt" for the 2nd modified revision
    and so on.  Each recipient

* the message files contain the message author, message timestamp, and file name of the previous
  message (not subnamed by revision mod # - just the root message num)

* How do we ensure the sequence of message file names doesn't have collisions between multiple
  conversers?

* Participants in a conversation are controlled by the upspin Access
  file.

* File contents must be cryptographically signed by the sender (and verified by the receiver).
  Where should the signature live?


- [num]
    - Root poster increases message count by one for each of their messages
    - When someone is invited to the conversation, they the inviter creates a new folder in their
        conversations directory and a file in that directory of the other conversation
        participant(s).  This file also contains an increment value that the participant uses for
        all their messages.  The increment for a new conversation participant is always set to the
        increment of the inviter plus 10.  Or something like this to help prevent collisions?

