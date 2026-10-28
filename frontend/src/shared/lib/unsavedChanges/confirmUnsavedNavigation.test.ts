import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import type { ModalFuncProps } from 'antd'
import { confirmUnsavedNavigation } from './confirmUnsavedNavigation'

describe('confirmUnsavedNavigation', () => {
  it('calls onConfirm immediately when not dirty', () => {
    let confirmed = false
    let modalCalled = false
    confirmUnsavedNavigation(
      () => false,
      (k) => k,
      () => {
        confirmed = true
      },
      () => {
        modalCalled = true
      }
    )
    assert.equal(confirmed, true)
    assert.equal(modalCalled, false)
  })

  it('opens modal when dirty', () => {
    let confirmed = false
    let modalOpts: ModalFuncProps | null = null
    confirmUnsavedNavigation(
      () => true,
      (k) => k,
      () => {
        confirmed = true
      },
      (opts) => {
        modalOpts = opts
      }
    )
    assert.equal(confirmed, false)
    assert.ok(modalOpts)
    ;(modalOpts as ModalFuncProps).onOk?.()
    assert.equal(confirmed, true)
  })
})
