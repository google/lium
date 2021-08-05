# -*- coding: utf-8 -*-
# Copyright (c) 2011 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Devserver module for handling update client requests."""

from __future__ import print_function

import logging
import os

from six.moves import urllib

import cherrypy  # pylint: disable=import-error

# TODO(crbug.com/872441): We try to import nebraska from different places
# because when we install the devserver, we copy the nebraska.py into the main
# directory. Once this bug is resolved, we can always import from nebraska
# directory.
try:
  from nebraska import nebraska
except ImportError:
  import nebraska


# Module-local log function.
def _Log(message, *args):
  return logging.info(message, *args)


class AutoupdateError(Exception):
  """Exception classes used by this module."""
  # pylint: disable=unnecessary-pass
  pass


def _ChangeUrlPort(url, new_port):
  """Return the URL passed in with a different port"""
  scheme, netloc, path, query, fragment = urllib.parse.urlsplit(url)
  host_port = netloc.split(':')

  if len(host_port) == 1:
    host_port.append(new_port)
  else:
    host_port[1] = new_port

  print(host_port)
  netloc = '%s:%s' % tuple(host_port)

  # pylint: disable=too-many-function-args
  return urllib.parse.urlunsplit((scheme, netloc, path, query, fragment))

def _NonePathJoin(*args):
  """os.path.join that filters None's from the argument list."""
  return os.path.join(*[x for x in args if x is not None])


class Autoupdate(object):
  """Class that contains functionality that handles Chrome OS update pings."""

  def __init__(self, xbuddy, static_dir=None):
    """Initializes the class.

    Args:
      xbuddy: The xbuddy path.
      static_dir: The path to the devserver static directory.
    """
    self.xbuddy = xbuddy
    self.static_dir = static_dir

  def GetDevserverUrl(self):
    """Returns the devserver url base."""
    x_forwarded_host = cherrypy.request.headers.get('X-Forwarded-Host')
    if x_forwarded_host:
      # Select the left most <ip>:<port> value so that the request is
      # forwarded correctly.
      x_forwarded_host = [x.strip() for x in x_forwarded_host.split(',')][0]
      hostname = 'http://' + x_forwarded_host
    else:
      hostname = cherrypy.request.base

    return hostname

  def GetStaticUrl(self):
    """Returns the static url base that should prefix all payload responses."""
    hostname = self.GetDevserverUrl()
    _Log('Handling update ping as %s', hostname)

    static_urlbase = '%s/static' % hostname
    _Log('Using static url base %s', static_urlbase)
    return static_urlbase

  def GetBuildID(self, label, board):
    """Find the build id of the given lable and board.

    Args:
      label: from update request
      board: from update request

    Returns:
      The build id of given label and board. e.g. reef-release/R88-13591.0.0

    Raises:
      AutoupdateError: If the update could not be found.
    """
    label = label or ''
    label_list = label.split('/')
    if label_list[0] == 'xbuddy':
      # If path explicitly calls xbuddy, pop off the tag.
      label_list.pop()
    x_label, _ = self.xbuddy.Translate(label_list, board=board)
    return x_label

  def HandleUpdatePing(self, data, label='', **kwargs):
    """Handles an update ping from an update client.

    Args:
      data: XML blob from client.
      label: optional label for the update.
      kwargs: The map of query strings passed to the /update API.

    Returns:
      Update payload message for client.
    """
    # Change the URL's string query dictionary provided by cherrypy to a valid
    # dictionary that has proper values for its keys. e.g. True instead of
    # 'True'.
    kwargs = nebraska.QueryDictToDict(kwargs)

    # Process attributes of the update check.
    request = nebraska.Request(data)
    if request.request_type == nebraska.Request.RequestType.EVENT:
      _Log('A non-update event notification received. Returning an ack.')
      n = nebraska.Nebraska()
      n.UpdateConfig(**kwargs)
      return n.GetResponseToRequest(request)

    _Log('Update Check Received.')

    try:
      build_id = self.GetBuildID(label, request.board)
      base_url = '/'.join((self.GetStaticUrl(), build_id))
      local_payload_dir = _NonePathJoin(self.static_dir, build_id)
    except AutoupdateError as e:
      # Raised if we fail to generate an update payload.
      _Log('Failed to process an update request, but we will defer to '
           'nebraska to respond with no-update. The error was %s', e)

    _Log('Responding to client to use url %s to get image', base_url)
    n = nebraska.Nebraska()
    n.UpdateConfig(update_payloads_address=base_url,
                   update_app_index=nebraska.AppIndex(local_payload_dir))
    return n.GetResponseToRequest(request)
