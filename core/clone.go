package core

func CloneRequest(req *Request) *Request {
	if req == nil {
		return nil
	}
	out := *req
	out.Messages = cloneMessages(req.Messages)
	out.System = cloneBlocks(req.System)
	out.Tools = cloneTools(req.Tools)
	if req.ToolChoice != nil {
		tc := *req.ToolChoice
		tc.Raw = append([]byte(nil), req.ToolChoice.Raw...)
		out.ToolChoice = &tc
	}
	out.Metadata = cloneMap(req.Metadata)
	out.Extensions = cloneMap(req.Extensions)
	return &out
}

func cloneMessages(in []Message) []Message {
	if in == nil {
		return nil
	}
	out := make([]Message, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].Content = cloneBlocks(in[i].Content)
		out[i].Extensions = cloneMap(in[i].Extensions)
	}
	return out
}

func cloneBlocks(in []ContentBlock) []ContentBlock {
	if in == nil {
		return nil
	}
	out := make([]ContentBlock, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].ToolInput = append([]byte(nil), in[i].ToolInput...)
		out[i].ToolResultContent = cloneBlocks(in[i].ToolResultContent)
		out[i].Extensions = cloneMap(in[i].Extensions)
	}
	return out
}

func cloneTools(in []Tool) []Tool {
	if in == nil {
		return nil
	}
	out := make([]Tool, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].InputSchema = cloneMap(in[i].InputSchema)
		out[i].Extensions = cloneMap(in[i].Extensions)
	}
	return out
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
