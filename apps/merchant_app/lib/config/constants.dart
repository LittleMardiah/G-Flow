const String kAppName = 'G-Flow Merchant';

String formatRupiah(int amount) {
  final negative = amount < 0;
  var s = amount.abs().toString();
  final buf = StringBuffer();
  for (var i = 0; i < s.length; i++) {
    buf.write(s[i]);
    final remaining = s.length - 1 - i;
    if (remaining > 0 && remaining % 3 == 0) buf.write('.');
  }
  return '${negative ? '-' : ''}Rp $buf';
}

String greetingForHour(DateTime now) {
  final hour = now.hour;
  if (hour < 11) return 'Good morning';
  if (hour < 15) return 'Good afternoon';
  if (hour < 19) return 'Good evening';
  return 'Good night';
}