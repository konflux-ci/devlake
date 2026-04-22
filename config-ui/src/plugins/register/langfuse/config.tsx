/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

import { IPluginConfig } from '@/types';

import Icon from './assets/icon.svg?react';
import { LangfuseDataScope } from './data-scope';

export const LangfuseConfig: IPluginConfig = {
  plugin: 'langfuse',
  name: 'Langfuse',
  icon: ({ color }) => <Icon fill={color} />,
  sort: 9,
  isBeta: true,
  connection: {
    docLink: 'https://langfuse.com/docs',
    initialValues: {
      endpoint: 'https://cloud.langfuse.com/',
    },
    fields: [
      'name',
      {
        key: 'endpoint',
        subLabel: 'The Langfuse API base URL. E.g. https://cloud.langfuse.com/ or your self-hosted instance URL.',
        defaultValue: 'https://cloud.langfuse.com/',
      },
      {
        key: 'username',
        label: 'Public Key',
        subLabel: 'The Langfuse public key for your project. Found in Langfuse under Settings > API Keys.',
        placeholder: 'pk-lf-...',
      },
      {
        key: 'password',
        label: 'Secret Key',
        subLabel: 'The Langfuse secret key for your project.',
        placeholder: 'sk-lf-...',
      },
      'proxy',
      {
        key: 'rateLimitPerHour',
        subLabel: 'Rate limit for Langfuse API requests. Default is 1000 requests/hour.',
        defaultValue: 1000,
      },
    ],
  },
  dataScope: {
    title: 'Projects',
    render: ({ connectionId, disabledItems, selectedItems, onChangeSelectedItems }) => (
      <LangfuseDataScope
        connectionId={connectionId}
        disabledItems={disabledItems}
        selectedItems={selectedItems as any}
        onChangeSelectedItems={onChangeSelectedItems}
      />
    ),
  },
};
