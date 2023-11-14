import {OperatePort} from './operate_port';
import {Ui} from './ui';

export class TestRunner {
  public isOpened = false;
  private CANCEL_CMD = '\x03\n';
  private ui: Ui;
  public dut = new OperatePort(0x18d1, 0x504a);
  constructor(ui: Ui, dut: OperatePort) {
    this.ui = ui;
    this.dut = dut;
  }
  public async openDutPort() {
    await this.dut.open();
    this.ui.addMessageToConsole('DutPort is opened');
  }
  public async closeDutPort() {
    await this.dut.close();
    this.ui.addMessageToConsole('DutPort is closed');
  }
  public async readData() {
    const chunk = await this.dut.read();
    return chunk;
  }
  public async copyScriptToDut(customScript: string) {
    const script = `#!/bin/bash -e
function workload () {
  ${customScript}
}
ectool chargecontrol idle
sleep 3
echo "start"
workload 1> ./test_out.log 2> ./test_err.log
echo "end"
sleep 3
echo "stop"
ectool chargecontrol normal\n`;
    await this.dut.write('cat > ./example.sh << EOF\n');
    await this.dut.write(btoa(script) + '\n');
    await this.dut.write('EOF\n');
  }
  public async executeScript() {
    await this.dut.write('base64 -d ./example.sh | bash\n');
  }
  public async executeCommand(s: string) {
    await this.dut.write(s);
  }
  public async sendCancel() {
    await this.dut.write(this.CANCEL_CMD);
  }
  public setupDisconnectEvent() {
    navigator.serial.addEventListener('disconnect', async () => {
      if (this.isOpened) {
        await this.dut.close();
        this.isOpened = false;
      }
    });
  }
}
