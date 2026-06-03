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

import { request } from '@/utils';

export interface IAgentReadyScopeConfig {
  id?: number;
  name?: string;
  branch: string;
  assessmentFilePath: string;
  excludeRepos: string;
  submissionsRepo: string;
  submissionsPath: string;
  submissionsBranch: string;
  submissionsConnectionId: number;
}

export const getScopeConfig = (id: number): Promise<IAgentReadyScopeConfig> =>
  request(`/plugins/agentready/scope-configs/${id}`);

export const createScopeConfig = (data: Partial<IAgentReadyScopeConfig>): Promise<IAgentReadyScopeConfig> =>
  request('/plugins/agentready/scope-configs', { method: 'post', data });

export const updateScopeConfig = (id: number, data: Partial<IAgentReadyScopeConfig>): Promise<IAgentReadyScopeConfig> =>
  request(`/plugins/agentready/scope-configs/${id}`, { method: 'patch', data });
