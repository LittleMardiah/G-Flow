import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../models/send_order.dart';
import '../../providers/send_order_provider.dart';

/// Send Package Form (ROADMAP 3.8 F & HALAMAN.txt 5.1) - multi-stop fare
/// breakdown. Fare rate mengikuti internal/send/service.go:
/// base 15.000 + jarak km x 3.000 + weight surcharge (>5 kg) + insurance
/// (declared value > 100.000, 1%, cap 50.000).
class SendPackageFormScreen extends ConsumerStatefulWidget {
  const SendPackageFormScreen({super.key});

  @override
  ConsumerState<SendPackageFormScreen> createState() => _SendPackageFormScreenState();
}

class _StopInput {
  final TextEditingController name;
  final TextEditingController phone;
  final TextEditingController address;
  final TextEditingController distance; // km
  _StopInput()
      : name = TextEditingController(),
        phone = TextEditingController(),
        address = TextEditingController(),
        distance = TextEditingController();
}

class _SendPackageFormScreenState extends ConsumerState<SendPackageFormScreen> {
  final TextEditingController _pickup = TextEditingController();
  final TextEditingController _weight = TextEditingController();
  final TextEditingController _declared = TextEditingController();
  final List<_StopInput> _stops = [_StopInput(), _StopInput()];
  String _packageType = 'STANDARD';
  String _paymentMethod = 'WALLET';

  static const double baseFare = 15000;
  static const int perKm = 3000;
  static const double weightThresholdKg = 5;
  static const int weightPerKg = 2000;
  static const double insuranceThreshold = 100000;
  static const double insuranceRate = 0.01;
  static const int insuranceMax = 50000;

  @override
  void dispose() {
    _pickup.dispose();
    _weight.dispose();
    _declared.dispose();
    for (final s in _stops) {
      s.name.dispose();
      s.phone.dispose();
      s.address.dispose();
      s.distance.dispose();
    }
    super.dispose();
  }

  double get _totalKm =>
      _stops.fold(0.0, (sum, s) => sum + (double.tryParse(s.distance.text) ?? 0));

  double get _weightKg => double.tryParse(_weight.text) ?? 0;

  double get _declaredValue => double.tryParse(_declared.text) ?? 0;

  /// Surcharge berat: hanya jika weight > 5 kg, +Rp 2.000 per kg kelebihan.
  int get _weightSurcharge {
    final excess = _weightKg - weightThresholdKg;
    if (excess <= 0) return 0;
    return (excess * weightPerKg).round();
  }

  /// Asuransi: 1% dari declared value (jika > 100.000), cap 50.000.
  int get _insuranceFee {
    if (_declaredValue <= insuranceThreshold) return 0;
    final fee = (_declaredValue * insuranceRate).round();
    return fee > insuranceMax ? insuranceMax : fee;
  }

  int get _fare {
    final base = baseFare.round();
    final distanceCharge = (_totalKm * perKm).round();
    return base + distanceCharge + _weightSurcharge + _insuranceFee;
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final submit = ref.watch(sendOrderSubmitProvider);

    return Scaffold(
      appBar: AppBar(title: const Text('Create Send')),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          _sectionTitle(theme, 'Package Details'),
          TextField(
            controller: _weight,
            keyboardType: const TextInputType.numberWithOptions(decimal: true),
            decoration: const InputDecoration(
              labelText: 'Berat (kg)',
              prefixIcon: Icon(Icons.scale_outlined),
            ),
          ),
          const SizedBox(height: 8),
          DropdownButtonFormField<String>(
            initialValue: _packageType,
            decoration: const InputDecoration(
              labelText: 'Tipe Paket',
              prefixIcon: Icon(Icons.inventory_2_outlined),
            ),
            items: const [
              DropdownMenuItem(value: 'STANDARD', child: Text('Standard')),
              DropdownMenuItem(value: 'FRAGILE', child: Text('Fragile')),
              DropdownMenuItem(value: 'LIQUID', child: Text('Liquid')),
              DropdownMenuItem(value: 'ELECTRONICS', child: Text('Electronics')),
            ],
            onChanged: (v) => setState(() => _packageType = v ?? 'STANDARD'),
          ),
          const SizedBox(height: 8),
          TextField(
            controller: _declared,
            keyboardType: const TextInputType.numberWithOptions(decimal: true),
            decoration: const InputDecoration(
              labelText: 'Nilai deklarasi (untuk asuransi, opsional)',
              prefixIcon: Icon(Icons.verified_user_outlined),
            ),
          ),
          const Divider(height: 28),
          _sectionTitle(theme, 'Pickup Location'),
          TextField(
            controller: _pickup,
            decoration: const InputDecoration(
              labelText: 'Alamat pickup',
              prefixIcon: Icon(Icons.my_location),
            ),
          ),
          const Divider(height: 28),
          _sectionTitle(theme, 'Delivery Stops'),
          for (var i = 0; i < _stops.length; i++) _stopCard(theme, i),
          const SizedBox(height: 8),
          OutlinedButton.icon(
            onPressed: _stops.length >= 3
                ? null
                : () => setState(() => _stops.add(_StopInput())),
            icon: const Icon(Icons.add_location_alt_outlined),
            label: const Text('Add Delivery Stop'),
          ),
          const Divider(height: 28),
          _sectionTitle(theme, 'Fare Breakdown'),
          _fareCard(theme),
          const Divider(height: 28),
          _sectionTitle(theme, 'Payment Method'),
          RadioGroup<String>(
            groupValue: _paymentMethod,
            onChanged: (v) => setState(() => _paymentMethod = v!),
            child: Column(
              children: [
                RadioListTile<String>(
                  dense: true,
                  contentPadding: EdgeInsets.zero,
                  title: const Text('PayPulse Wallet'),
                  value: 'WALLET',
                ),
                RadioListTile<String>(
                  dense: true,
                  contentPadding: EdgeInsets.zero,
                  title: const Text('Cash on Delivery'),
                  value: 'CASH',
                ),
              ],
            ),
          ),
          if (submit.error != null)
            Padding(
              padding: const EdgeInsets.only(top: 12),
              child: Text(submit.error!,
                  style: TextStyle(color: theme.colorScheme.error)),
            ),
        ],
      ),
      bottomNavigationBar: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: FilledButton.icon(
            style: FilledButton.styleFrom(minimumSize: const Size.fromHeight(52)),
            onPressed: submit.isSubmitting ? null : _submit,
            icon: submit.isSubmitting
                ? const SizedBox(
                    width: 18,
                    height: 18,
                    child: CircularProgressIndicator(strokeWidth: 2))
                : const Icon(Icons.send_outlined),
            label: Text(submit.isSubmitting
                ? 'Memproses…'
                : 'CONFIRM & SEND · ${_formatRupiah(_fare)}'),
          ),
        ),
      ),
    );
  }

  Widget _sectionTitle(ThemeData theme, String title) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Text(title,
          style: theme.textTheme.titleSmall
              ?.copyWith(fontWeight: FontWeight.bold)),
    );
  }

  Widget _stopCard(ThemeData theme, int index) {
    final s = _stops[index];
    return Card(
      margin: const EdgeInsets.only(bottom: 10),
      elevation: 0,
      color: theme.colorScheme.surfaceContainerLow,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          children: [
            Row(
              children: [
                Expanded(
                  child: Text('Stop ${index + 1}',
                      style: theme.textTheme.titleSmall),
                ),
                if (_stops.length > 1)
                  IconButton(
                    visualDensity: VisualDensity.compact,
                    icon: const Icon(Icons.delete_outline, color: Colors.red),
                    onPressed: () => setState(() => _stops.removeAt(index)),
                  ),
              ],
            ),
            TextField(
              controller: s.name,
              decoration: const InputDecoration(labelText: 'Nama penerima'),
            ),
            TextField(
              controller: s.phone,
              keyboardType: TextInputType.phone,
              decoration: const InputDecoration(labelText: 'No. HP'),
            ),
            TextField(
              controller: s.address,
              decoration: const InputDecoration(labelText: 'Alamat tujuan'),
            ),
            TextField(
              controller: s.distance,
              keyboardType: const TextInputType.numberWithOptions(decimal: true),
              decoration: const InputDecoration(labelText: 'Estimasi jarak (km)'),
            ),
          ],
        ),
      ),
    );
  }

  Widget _fareCard(ThemeData theme) {
    final stops = _stops
        .asMap()
        .entries
        .where((e) => (double.tryParse(e.value.distance.text) ?? 0) > 0)
        .toList();
    return Card(
      margin: EdgeInsets.zero,
      elevation: 0,
      color: theme.colorScheme.surfaceContainerLow,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          children: [
            if (stops.isNotEmpty)
              for (final entry in stops)
                _fareRow(
                  'Stop ${entry.key + 1} (${entry.value.distance.text} km)',
                  (double.parse(entry.value.distance.text) * perKm).round(),
                ),
            _fareRow('Base Fare', baseFare.round()),
            if (_weightSurcharge > 0)
              _fareRow('Weight Surcharge', _weightSurcharge),
            if (_insuranceFee > 0) _fareRow('Asuransi', _insuranceFee),
            const Divider(),
            _fareRow('Total', _fare, bold: true),
          ],
        ),
      ),
    );
  }

  Widget _fareRow(String label, int value, {bool bold = false}) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 3),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(label,
              style: bold
                  ? Theme.of(context).textTheme.titleSmall
                  : Theme.of(context).textTheme.bodyMedium),
          Text(_formatRupiah(value),
              style: bold
                  ? Theme.of(context).textTheme.titleSmall
                      ?.copyWith(fontWeight: FontWeight.bold)
                  : null),
        ],
      ),
    );
  }

  Future<void> _submit() async {
    if (_pickup.text.trim().isEmpty) {
      _snack('Masukkan alamat pickup.');
      return;
    }
    if (_weightKg <= 0) {
      _snack('Masukkan berat paket yang valid.');
      return;
    }
    final validStops = _stops.where((s) =>
        s.name.text.trim().isNotEmpty &&
        s.address.text.trim().isNotEmpty &&
        (double.tryParse(s.distance.text) ?? 0) > 0);
    final stopsList = validStops.toList();
    if (stopsList.isEmpty) {
      _snack('Tambahkan minimal satu stop pengiriman yang lengkap.');
      return;
    }

    final stops = <Map<String, dynamic>>[];
    for (final s in stopsList) {
      stops.add({
        'recipient_name': s.name.text.trim(),
        'recipient_phone': s.phone.text.trim(),
        'delivery_address': s.address.text.trim(),
        'delivery_lat': 0,
        'delivery_lng': 0,
      });
    }

    final orderId = await ref.read(sendOrderSubmitProvider.notifier).submit(
          CreateSendOrderInput(
            packageType: _packageType,
            stops: stops,
            paymentMethod: _paymentMethod,
            pickupAddress: _pickup.text.trim(),
            weightKg: _weightKg,
            declaredValue: _declaredValue,
          ),
        );
    if (orderId == null || !mounted) return;
    Navigator.of(context).pushReplacementNamed(
      '/send-tracking',
      arguments: SendOrderTrackingArgs(
        orderId: orderId,
        totalAmount: _fare,
        packageType: _packageType,
        paymentMethod: _paymentMethod,
      ),
    );
  }

  void _snack(String msg) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(msg)));
  }
}

String _formatRupiah(num value) {
  final digits = value.round().toString();
  final buffer = StringBuffer();
  for (var i = 0; i < digits.length; i++) {
    if (i > 0 && (digits.length - i) % 3 == 0) buffer.write('.');
    buffer.write(digits[i]);
  }
  return 'Rp $buffer';
}