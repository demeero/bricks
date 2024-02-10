package httpbrick

type otelMeterAttrsOpts struct {
	URI          bool
	Code         bool
	AttrsFromCtx bool
	AttrsToCtx   bool
}

type otelMeterMetricsOpts struct {
	Names             otelMetricNames
	ReqDuration       bool
	ReqCounter        bool
	ActiveReqsCounter bool
	ReqSize           bool
	RespSize          bool
}

type otelMetricNames struct {
	ReqDurationHist   string
	ReqCounter        string
	ReqSizeHist       string
	RespSizeHist      string
	ActiveReqsCounter string
}

type otelMeterOpts struct {
	Skipper Skipper
	Metrics otelMeterMetricsOpts
	Attrs   otelMeterAttrsOpts
}

type OTelMeterMWOption func(*otelMeterOpts)

func WithURIMeterAttr() OTelMeterMWOption {
	return func(n *otelMeterOpts) {
		n.Attrs.URI = true
	}
}

func WithCodeMeterAttr() OTelMeterMWOption {
	return func(n *otelMeterOpts) {
		n.Attrs.Code = true
	}
}

func WithoutAttrsToCtx() OTelMeterMWOption {
	return func(n *otelMeterOpts) {
		n.Attrs.AttrsToCtx = false
	}
}

func WithoutAttrsFromCtx() OTelMeterMWOption {
	return func(n *otelMeterOpts) {
		n.Attrs.AttrsFromCtx = false
	}
}

func WithoutReqDurationMetric() OTelMeterMWOption {
	return func(n *otelMeterOpts) {
		n.Metrics.ReqDuration = false
	}
}

func WithoutReqCounterMetric() OTelMeterMWOption {
	return func(n *otelMeterOpts) {
		n.Metrics.ReqCounter = false
	}
}

func WithoutActiveReqsCounterMetric() OTelMeterMWOption {
	return func(n *otelMeterOpts) {
		n.Metrics.ActiveReqsCounter = false
	}
}

func WithoutReqSizeMetric() OTelMeterMWOption {
	return func(n *otelMeterOpts) {
		n.Metrics.ReqSize = false
	}
}

func WithoutRespSizeMetric() OTelMeterMWOption {
	return func(n *otelMeterOpts) {
		n.Metrics.RespSize = false
	}
}

func WithSkipper(skipper Skipper) OTelMeterMWOption {
	return func(n *otelMeterOpts) {
		n.Skipper = skipper
	}
}

func WithMetricNames(names otelMetricNames) OTelMeterMWOption {
	return func(opts *otelMeterOpts) {
		if names.ReqDurationHist != "" {
			opts.Metrics.Names.ReqDurationHist = names.ReqDurationHist
		}
		if names.ReqCounter != "" {
			opts.Metrics.Names.ReqCounter = names.ReqCounter
		}
		if names.ReqSizeHist != "" {
			opts.Metrics.Names.ReqSizeHist = names.ReqSizeHist
		}
		if names.RespSizeHist != "" {
			opts.Metrics.Names.RespSizeHist = names.RespSizeHist
		}
		if names.ActiveReqsCounter != "" {
			opts.Metrics.Names.ActiveReqsCounter = names.ActiveReqsCounter
		}
	}
}
