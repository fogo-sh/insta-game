package agent

import "github.com/fogo-sh/insta-game/internal/protocol"

func queryServer(c Config) *protocol.ServerInfo {
	var info *protocol.ServerInfo
	var err error
	switch c.Protocol {
	case "bzflag":
		info, err = protocol.QueryBZFlag(c.GamePort)
	case "quake1":
		info, err = protocol.QueryQuake1(c.GamePort)
	case "quake2":
		info, err = protocol.QueryQuake2(c.GamePort)
	case "quake3":
		info, err = protocol.QueryQuake3(c.GamePort)
	case "ut99":
		info, err = protocol.QueryUT99(c.GamePort)
	case "xonotic":
		info, err = protocol.QueryXonotic(c.GamePort)
	default:
		return nil
	}
	if err != nil {
		return nil
	}
	return info
}
