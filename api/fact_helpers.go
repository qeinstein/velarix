package api

import "velarix/core"

func moveProvenanceFromPayloadToMetadata(fact *core.Fact) {
	if fact == nil || fact.Payload == nil {
		return
	}
	p, ok := fact.Payload["_provenance"]
	if !ok {
		return
	}
	delete(fact.Payload, "_provenance")
	if fact.Metadata == nil {
		fact.Metadata = make(map[string]interface{})
	}
	fact.Metadata["_provenance"] = p
}
