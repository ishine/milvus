// Copyright (C) 2019-2020 Zilliz. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance
// with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License
// is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
// or implied. See the License for the specific language governing permissions and limitations under the License.

package rocksmq

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClient(t *testing.T) {
	client, err := NewClient(ClientOptions{})
	assert.NotNil(t, client)
	assert.Nil(t, err)
}

func TestCreateProducer(t *testing.T) {
	client, err := NewClient(ClientOptions{
		Server: newMockRocksMQ(),
	})
	assert.NoError(t, err)

	producer, err := client.CreateProducer(ProducerOptions{
		Topic: newTopicName(),
	})
	assert.Error(t, err)
	assert.Nil(t, producer)

	client.Close()
}

func TestSubscribe(t *testing.T) {
	client, err := NewClient(ClientOptions{
		Server: newMockRocksMQ(),
	})
	assert.NoError(t, err)

	consumer, err := client.Subscribe(ConsumerOptions{
		Topic:            newTopicName(),
		SubscriptionName: newConsumerName(),
	})
	assert.Error(t, err)
	assert.Nil(t, consumer)

	client.Close()
}
