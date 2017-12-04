package lats_test

import (
	"crypto/tls"
	"time"

	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"
	"github.com/cloudfoundry/noaa/consumer"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/golang/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	cfSetupTimeOut     = 10 * time.Second
	cfPushTimeOut      = 2 * time.Minute
	defaultMemoryLimit = "256MB"
)

var _ = Describe("Logs", func() {
	Describe("emit v1 and consume via traffic controller", func() {
		It("gets through recent logs", func() {
			env := createLogEnvelopeV1("Recent log message", "foo")
			EmitToMetronV1(env)

			tlsConfig := &tls.Config{InsecureSkipVerify: true}
			consumer := consumer.New(config.DopplerEndpoint, tlsConfig, nil)

			getRecentLogs := func() []*events.LogMessage {
				envelopes, err := consumer.RecentLogs("foo", "")
				Expect(err).NotTo(HaveOccurred())
				return envelopes
			}

			Eventually(getRecentLogs).Should(ContainElement(env.LogMessage))
		})

		It("sends log messages for a specific app through the stream endpoint", func() {
			msgChan, errorChan := ConnectToStream("foo")

			env := createLogEnvelopeV1("Stream message", "foo")
			EmitToMetronV1(env)

			receivedEnvelope, err := FindMatchingEnvelopeByID("foo", msgChan)
			Expect(err).NotTo(HaveOccurred())

			Expect(receivedEnvelope.LogMessage).To(Equal(env.LogMessage))

			Expect(errorChan).To(BeEmpty())
		})
	})

	Describe("emit v2 and consume via traffic controller", func() {
		It("gets through recent logs", func() {
			env := createLogEnvelopeV2("Recent log message", "foo")
			EmitToMetronV2(env)

			tlsConfig := &tls.Config{InsecureSkipVerify: true}
			consumer := consumer.New(config.DopplerEndpoint, tlsConfig, nil)

			getRecentLogs := func() []*events.LogMessage {
				envelopes, err := consumer.RecentLogs("foo", "")
				Expect(err).NotTo(HaveOccurred())
				return envelopes
			}

			v1EnvLogMsg := &events.LogMessage{
				Message:        env.GetLog().Payload,
				MessageType:    events.LogMessage_OUT.Enum(),
				Timestamp:      proto.Int64(env.Timestamp),
				AppId:          proto.String(env.SourceId),
				SourceType:     proto.String(""),
				SourceInstance: proto.String(""),
			}

			Eventually(getRecentLogs).Should(ContainElement(v1EnvLogMsg))
		})

		It("sends log messages for a specific app through the stream endpoint", func() {
			msgChan, errorChan := ConnectToStream("foo-stream")

			env := createLogEnvelopeV2("Stream message", "foo-stream")
			EmitToMetronV2(env)

			receivedEnvelope, err := FindMatchingEnvelopeByID("foo-stream", msgChan)
			Expect(err).NotTo(HaveOccurred())

			v1EnvLogMsg := &events.LogMessage{
				Message:        env.GetLog().Payload,
				MessageType:    events.LogMessage_OUT.Enum(),
				Timestamp:      proto.Int64(env.Timestamp),
				AppId:          proto.String(env.SourceId),
				SourceType:     proto.String(""),
				SourceInstance: proto.String(""),
			}

			Expect(receivedEnvelope.LogMessage).To(Equal(v1EnvLogMsg))

			Expect(errorChan).To(BeEmpty())
		})
	})

	Describe("emit v1 and consume via reverse log proxy", func() {
		It("sends log messages through rlp", func() {
			msgChan := ReadFromRLP("rlp-stream-foo", false)

			env := createLogEnvelopeV1("Stream message", "rlp-stream-foo")
			EmitToMetronV1(env)

			v2EnvLog := &v2.Log{
				Payload: env.GetLogMessage().Message,
				Type:    v2.Log_OUT,
			}

			var outEnv *v2.Envelope
			Eventually(msgChan, 5).Should(Receive(&outEnv))
			Expect(outEnv.GetLog()).To(Equal(v2EnvLog))
		})

		It("sends log messages through rlp with preferred tags", func() {
			msgChan := ReadFromRLP("rlp-stream-foo", true)

			env := createLogEnvelopeV1("Stream message", "rlp-stream-foo")
			EmitToMetronV1(env)

			v2EnvLog := &v2.Log{
				Payload: env.GetLogMessage().Message,
				Type:    v2.Log_OUT,
			}

			var outEnv *v2.Envelope
			Eventually(msgChan, 5).Should(Receive(&outEnv))
			Expect(outEnv.GetLog()).To(Equal(v2EnvLog))
		})
	})

	Describe("emit v2 and consume via reverse log proxy", func() {
		It("sends log messages through rlp", func() {
			msgChan := ReadFromRLP("rlp-stream-foo", false)

			env := createLogEnvelopeV2("Stream message", "rlp-stream-foo")
			EmitToMetronV2(env)

			var outEnv *v2.Envelope
			Eventually(msgChan, 5).Should(Receive(&outEnv))
			Expect(outEnv.GetLog()).To(Equal(env.GetLog()))
		})
	})
})

func createLogEnvelopeV1(message, appID string) *events.Envelope {
	return &events.Envelope{
		EventType: events.Envelope_LogMessage.Enum(),
		Origin:    proto.String(OriginName),
		Timestamp: proto.Int64(time.Now().UnixNano()),
		LogMessage: &events.LogMessage{
			Message:     []byte(message),
			MessageType: events.LogMessage_OUT.Enum(),
			Timestamp:   proto.Int64(time.Now().UnixNano()),
			AppId:       proto.String(appID),
		},
	}
}

func createLogEnvelopeV2(message, appID string) *v2.Envelope {
	return &v2.Envelope{
		SourceId:  appID,
		Timestamp: time.Now().UnixNano(),
		DeprecatedTags: map[string]*v2.Value{
			"origin": {
				Data: &v2.Value_Text{
					Text: OriginName,
				},
			},
		},
		Message: &v2.Envelope_Log{
			Log: &v2.Log{
				Payload: []byte(message),
				Type:    v2.Log_OUT,
			},
		},
	}
}
