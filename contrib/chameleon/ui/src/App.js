// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

import React from 'react';

import PortButtons from './component/portButtons';

const App = () => {
  return (
    <div>
      <p>
        Welcome to <code>Chamelium Control Center</code>
      </p>
      <PortButtons />
    </div>
  );
};

export default App;