# Object Calisthenics in Go: Before/After

## 1) One indentation level + no else

Before:
```go
func (u *SendMessageUseCase) Execute(ctx context.Context, in SendMessageInput) error {
    if in.SellerID != "" {
        if in.ConversationID != "" {
            if in.Body != "" {
                return u.repo.Save(ctx, in)
            } else {
                return ErrEmptyBody
            }
        } else {
            return ErrInvalidConversationID
        }
    } else {
        return ErrInvalidSellerID
    }
}
```

After:
```go
func (u *SendMessageUseCase) Execute(ctx context.Context, in SendMessageInput) error {
    if in.SellerID == "" {
        return ErrInvalidSellerID
    }
    if in.ConversationID == "" {
        return ErrInvalidConversationID
    }
    if in.Body == "" {
        return ErrEmptyBody
    }
    return u.repo.Save(ctx, in)
}
```

## 2) Strong types over primitives

Before:
```go
func BuildStreamKey(sellerID, conversationID string) string {
    return sellerID + ":" + conversationID
}
```

After:
```go
type SellerID string
type ConversationID string

func BuildStreamKey(sellerID SellerID, conversationID ConversationID) string {
    return string(sellerID) + ":" + string(conversationID)
}
```

## 3) Limit arguments with input struct

Before:
```go
func NewCommand(sellerID, requestID, conversationID, body, channel, locale string) Command
```

After:
```go
type NewCommandInput struct {
    SellerID       SellerID
    RequestID      RequestID
    ConversationID ConversationID
    Body           string
    Channel        string
    Locale         string
}

func NewCommand(in NewCommandInput) Command
```

## 4) Encapsulate collections with invariants

Before:
```go
type Conversation struct {
    Messages []Message
}
```

After:
```go
type Messages struct {
    items []Message
}

func (m *Messages) Append(msg Message) error {
    if msg.Seq <= 0 {
        return ErrInvalidSeq
    }
    m.items = append(m.items, msg)
    return nil
}

type Conversation struct {
    Messages Messages
}
```
